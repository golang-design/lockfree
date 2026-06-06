// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf

import "sync/atomic"

// The wait-free stack is built with Herlihy's wait-free universal construction
// (Herlihy & Shavit, "The Art of Multiprocessor Programming", section 6.4):
// operations are linearized into a single consensus-ordered list, and every
// participant helps append announced operations in round-robin order so no
// operation can be starved.
//
// The sequential object is represented persistently: each operation node stores
// the immutable stack (a linked list of stackCell) that results from applying it
// to its predecessor's stack, together with that operation's response. This
// makes applying an operation O(1) (no log replay) and lets a late-registering
// participant or the garbage collector work from the current frontier instead of
// the whole history.

type stackOp int8

const (
	stackPush stackOp = iota
	stackPop
)

// stackCell is one node of an immutable (persistent) stack. Cells are shared
// across versions and never mutated after construction.
type stackCell[T any] struct {
	val  T
	next *stackCell[T]
}

// stackResult is the state and response produced by applying one operation:
// top is the resulting stack, and (val, ok) is a pop's response.
type stackResult[T any] struct {
	top *stackCell[T]
	val T
	ok  bool
}

// stackNode is an entry in the consensus-ordered operation list. res is always
// published before seq, so a non-zero seq guarantees res is visible.
type stackNode[T any] struct {
	op   stackOp
	arg  T
	next atomic.Pointer[stackNode[T]]   // consensus winner for the successor
	res  atomic.Pointer[stackResult[T]] // state+response after applying this op
	seq  atomic.Int64                   // position in the list; 0 until linked
}

// Stack is a wait-free LIFO stack for multiple concurrent participants, built on
// the wait-free universal construction.
//
// Progress guarantee: wait-free. Each operation announces itself, then every
// participant helps append announced operations in round-robin order, so any
// announced operation is linked within a bounded number of steps (O(maxHandles))
// regardless of scheduling. There are no locks and no operation waits on
// another's completion.
//
// Memory reclamation relies on Go's garbage collector. The construction keeps no
// permanent reference to the head of the operation list: the per-participant
// cursors and an anchor advance toward the frontier, so once every participant
// has moved past an operation node it (and the persistent-stack cells it alone
// kept alive) becomes unreachable and is collected.
//
// Like the wait-free Queue, participants must register because the helping
// arrays are indexed by participant id: obtain one Handle per goroutine (a
// StackHandle) up to maxHandles. Slots are not reclaimable (maxHandles is the
// lifetime total), which suits a bounded, long-lived worker pool. Each operation
// is O(maxHandles).
type Stack[T any] struct {
	anchor   atomic.Pointer[stackNode[T]]   // a recent node; advances to free the prefix
	head     []atomic.Pointer[stackNode[T]] // per-participant frontier (nil until registered)
	announce []atomic.Pointer[stackNode[T]] // per-participant current operation
	cursor   atomic.Int64                   // handle allocation cursor
	n        int
}

// NewStack creates a wait-free stack. maxHandles is the total number of Handle
// registrations allowed over the stack's lifetime (slots are not reclaimable);
// it must be at least 1.
func NewStack[T any](maxHandles int) *Stack[T] {
	if maxHandles < 1 {
		panic("wf: NewStack maxHandles must be >= 1")
	}
	sentinel := &stackNode[T]{}
	sentinel.res.Store(&stackResult[T]{}) // empty stack
	sentinel.seq.Store(1)
	s := &Stack[T]{
		head:     make([]atomic.Pointer[stackNode[T]], maxHandles),
		announce: make([]atomic.Pointer[stackNode[T]], maxHandles),
		n:        maxHandles,
	}
	s.anchor.Store(sentinel)
	return s
}

// StackHandle is a participant's access point to a Stack. It is bound to one slot
// in the helping arrays and is NOT safe for concurrent use by multiple
// goroutines: acquire one per goroutine.
type StackHandle[T any] struct {
	s   *Stack[T]
	tid int
}

// Handle registers a new participant and returns its StackHandle. Each call
// permanently consumes one of the maxHandles slots. It panics if more than
// maxHandles handles are requested.
func (s *Stack[T]) Handle() *StackHandle[T] {
	id := s.cursor.Add(1) - 1
	if id >= int64(s.n) {
		panic("wf: too many handles (exceeds maxHandles passed to NewStack)")
	}
	// Register lazily at the current frontier, not the original sentinel, so an
	// idle slot never pins the operation history.
	frontier := s.anchor.Load()
	s.head[id].Store(frontier)
	s.announce[id].Store(frontier)
	return &StackHandle[T]{s: s, tid: int(id)}
}

// Push pushes v onto the stack.
func (h *StackHandle[T]) Push(v T) { h.s.apply(h.tid, stackPush, v) }

// Pop removes and returns the value on top of the stack. The second return
// value is false if the stack was empty during the operation.
func (h *StackHandle[T]) Pop() (T, bool) {
	var zero T
	r := h.s.apply(h.tid, stackPop, zero)
	return r.val, r.ok
}

// maxFrontier returns the furthest known operation node (largest seq), used as
// the starting point for a new operation.
func (s *Stack[T]) maxFrontier() *stackNode[T] {
	best := s.anchor.Load()
	bestSeq := best.seq.Load()
	for i := 0; i < s.n; i++ {
		if nd := s.head[i].Load(); nd != nil {
			if sq := nd.seq.Load(); sq > bestSeq {
				best, bestSeq = nd, sq
			}
		}
	}
	return best
}

func (s *Stack[T]) apply(tid int, op stackOp, arg T) *stackResult[T] {
	mine := &stackNode[T]{op: op, arg: arg}
	s.announce[tid].Store(mine)
	s.head[tid].Store(s.maxFrontier())

	for mine.seq.Load() == 0 {
		before := s.head[tid].Load()
		// Round-robin: help the participant whose turn follows this position,
		// otherwise push our own operation.
		var prefer *stackNode[T]
		helpIdx := (before.seq.Load() + 1) % int64(s.n)
		if cand := s.announce[helpIdx].Load(); cand != nil && cand.seq.Load() == 0 {
			prefer = cand
		} else {
			prefer = mine
		}
		before.next.CompareAndSwap(nil, prefer) // consensus on the successor
		after := before.next.Load()
		if after.seq.Load() == 0 {
			s.publish(before, after)
		}
		s.head[tid].Store(after)
	}

	r := mine.res.Load()
	s.head[tid].Store(mine)
	s.advanceAnchor(mine)
	return r
}

// publish computes after's result from before's result and publishes it, then
// its sequence number. res is set before seq so a reader that sees seq != 0 is
// guaranteed to see res. Both stores are idempotent: every helper computes the
// same values.
func (s *Stack[T]) publish(before, after *stackNode[T]) {
	br := before.res.Load()
	var nr *stackResult[T]
	switch after.op {
	case stackPush:
		nr = &stackResult[T]{top: &stackCell[T]{val: after.arg, next: br.top}}
	default: // stackPop
		if br.top == nil {
			nr = &stackResult[T]{} // empty: ok == false
		} else {
			nr = &stackResult[T]{top: br.top.next, val: br.top.val, ok: true}
		}
	}
	after.res.CompareAndSwap(nil, nr)
	after.seq.CompareAndSwap(0, before.seq.Load()+1)
}

// advanceAnchor moves the anchor forward to n if n is further along, letting the
// garbage collector reclaim the now-unreachable prefix of the operation list.
func (s *Stack[T]) advanceAnchor(n *stackNode[T]) {
	for {
		cur := s.anchor.Load()
		if n.seq.Load() <= cur.seq.Load() {
			return
		}
		if s.anchor.CompareAndSwap(cur, n) {
			return
		}
	}
}
