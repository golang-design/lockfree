// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import (
	"math/rand/v2"
	"sync/atomic"
)

const (
	elimDefaultSlots = 16
	elimDefaultSpins = 32
)

// exState is the state of an exchanger slot.
type exState int32

const (
	exEmpty exState = iota
	exWaiting
	exBusy
)

// slotVal is an immutable (item, state) pair stored behind an atomic pointer; a
// fresh one is allocated on every post, which gives ABA-freedom.
type slotVal[T any] struct {
	item  *T
	state exState
}

// exchanger is a lock-free exchanger (Hendler, Shavit & Yerushalmi 2004): two
// goroutines that meet in a slot swap their offered items. A push offers a
// non-nil *value and a pop offers nil, so the stack can tell a real push/pop
// elimination (it got the complementary kind back) from a same-kind collision.
type exchanger[T any] struct {
	p atomic.Pointer[slotVal[T]]
}

// exchange makes one attempt to swap my with a partner, spinning up to spins
// iterations while waiting. It returns the partner's offered item and whether a
// swap happened.
func (e *exchanger[T]) exchange(my *T, spins int) (partner *T, matched bool) {
	cur := e.p.Load()
	st := exEmpty
	if cur != nil {
		st = cur.state
	}
	switch st {
	case exEmpty:
		// Post ourselves as the waiter.
		mine := &slotVal[T]{item: my, state: exWaiting}
		if !e.p.CompareAndSwap(cur, mine) {
			return nil, false
		}
		for i := 0; i < spins; i++ {
			if c := e.p.Load(); c != mine { // a matcher replaced us with a BUSY value
				if c == nil {
					return nil, false
				}
				e.p.CompareAndSwap(c, nil) // we are the single resetter
				return c.item, true
			}
		}
		if e.p.CompareAndSwap(mine, nil) {
			return nil, false // timed out, withdrew cleanly
		}
		// Withdrawal failed: a matcher arrived at the last moment.
		c := e.p.Load()
		if c == nil {
			return nil, false
		}
		e.p.CompareAndSwap(c, nil)
		return c.item, true
	case exWaiting:
		// Match the waiter: install BUSY carrying our item and take theirs.
		busy := &slotVal[T]{item: my, state: exBusy}
		if e.p.CompareAndSwap(cur, busy) {
			return cur.item, true
		}
		return nil, false
	default: // exBusy: another exchange is in progress
		return nil, false
	}
}

// EliminationStack is a lock-free LIFO stack with an elimination backoff array
// (Hendler, Shavit & Yerushalmi 2004). It is the same ADT as Stack (Treiber) but
// scales better under high contention: when a push and a pop collide on the
// central stack they may pair off through the elimination array and cancel out
// without touching the top pointer.
//
// Progress guarantee: lock-free. The central stack is a Treiber stack; a failed
// top CAS means another operation's central CAS succeeded, so the system always
// makes progress. Elimination uses only bounded spinning and falls back to the
// central stack, so it never blocks progress.
//
// Memory reclamation relies on Go's garbage collector. Length counts only
// elements resident on the central stack; an eliminated push/pop pair touches
// neither end.
type EliminationStack[T any] struct {
	top          atomic.Pointer[directItem[T]]
	length       atomic.Uint64
	slots        []exchanger[T]
	spins        int
	eliminations atomic.Uint64 // successful eliminations; observed by white-box tests
}

// NewEliminationStack creates an empty elimination-backoff stack.
func NewEliminationStack[T any]() *EliminationStack[T] {
	return newEliminationStack[T](elimDefaultSlots, elimDefaultSpins)
}

func newEliminationStack[T any](slots, spins int) *EliminationStack[T] {
	return &EliminationStack[T]{slots: make([]exchanger[T], slots), spins: spins}
}

// Push pushes a value on top of the stack.
func (s *EliminationStack[T]) Push(v T) {
	node := &directItem[T]{v: v}
	for {
		top := s.top.Load()
		node.next.Store(top)
		if s.top.CompareAndSwap(top, node) {
			s.length.Add(1)
			return
		}
		// Contention: try to hand our value directly to a colliding pop.
		slot := &s.slots[rand.IntN(len(s.slots))]
		if partner, matched := slot.exchange(&v, s.spins); matched && partner == nil {
			s.eliminations.Add(1)
			return
		}
	}
}

// Pop removes and returns the value on top of the stack. The second return value
// is false if the stack was empty during the operation.
func (s *EliminationStack[T]) Pop() (v T, ok bool) {
	for {
		top := s.top.Load()
		if top == nil {
			return v, false
		}
		if s.top.CompareAndSwap(top, top.next.Load()) {
			s.length.Add(^uint64(0))
			return top.v, true
		}
		// Contention: try to take a value directly from a colliding push.
		slot := &s.slots[rand.IntN(len(s.slots))]
		if partner, matched := slot.exchange(nil, s.spins); matched && partner != nil {
			s.eliminations.Add(1)
			return *partner, true
		}
	}
}

// Length returns the number of elements currently resident on the stack.
func (s *EliminationStack[T]) Length() uint64 {
	return s.length.Load()
}
