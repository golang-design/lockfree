// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import "runtime"

// Deque is a lock-free double-ended queue backed by a doubly linked list
// (Sundell & Tsigas, "Lock-Free and Practical Doubly Linked List-Based Deques
// Using Single-Word Compare-and-Swap", OPODIS 2004). Each node's next and prev
// links carry a deletion mark; insertion and deletion update the next pointer
// first and then lazily correct prev pointers, with concurrent operations
// helping complete one another.
//
// Progress guarantee: lock-free. Push and Pop at either end are CAS loops that,
// on interference, help a conflicting operation finish (HelpInsert / HelpDelete)
// and retry, so the system always makes progress.
//
// Memory reclamation relies on Go's garbage collector. The original algorithm
// uses lock-free reference counting (COPY/REL/DEREF and RemoveCrossReference to
// break cyclic garbage); under the GC all of that is unnecessary, so this port
// keeps only the two dereference variants (one returns nil when the link is
// marked, one ignores the mark) and the control flow.
type Deque[T any] struct {
	head *denode[T]
	tail *denode[T]
}

type denode[T any] struct {
	value T
	prev  atomicMarkable[denode[T]]
	next  atomicMarkable[denode[T]]
}

// NewDeque returns an empty lock-free deque.
func NewDeque[T any]() *Deque[T] {
	head := &denode[T]{}
	tail := &denode[T]{}
	head.next.set(tail, false)
	head.prev.set(head, false) // never dereferenced; head is never deleted
	tail.prev.set(head, false)
	tail.next.set(tail, false) // never dereferenced; tail is never deleted
	return &Deque[T]{head: head, tail: tail}
}

// derefActive returns the link target, or nil if the link is marked (DEREF).
func derefActive[N any](m *atomicMarkable[N]) *N {
	r, mark := m.get()
	if mark {
		return nil
	}
	return r
}

// PushFront inserts v at the left end of the deque (PushLeft).
func (d *Deque[T]) PushFront(v T) {
	node := &denode[T]{value: v}
	prev := d.head
	next := derefActive(&prev.next)
	for {
		if r, mark := prev.next.get(); r != next || mark { // prev.next != (next,F)
			next = derefActive(&prev.next)
			continue
		}
		node.prev.set(prev, false)
		node.next.set(next, false)
		if prev.next.compareAndSet(next, node, false, false) {
			break
		}
		runtime.Gosched()
	}
	d.pushCommon(node, next)
}

// PushBack inserts v at the right end of the deque (PushRight).
func (d *Deque[T]) PushBack(v T) {
	node := &denode[T]{value: v}
	next := d.tail
	prev := derefActive(&next.prev)
	for {
		if r, mark := prev.next.get(); r != next || mark { // prev.next != (next,F)
			prev = d.helpInsert(prev, next)
			continue
		}
		node.prev.set(prev, false)
		node.next.set(next, false)
		if prev.next.compareAndSet(next, node, false, false) {
			break
		}
		runtime.Gosched()
	}
	d.pushCommon(node, next)
}

// pushCommon updates the prev pointer of the node that follows a freshly
// inserted node (PushCommon).
func (d *Deque[T]) pushCommon(node, next *denode[T]) {
	for {
		link1Ref, link1Mark := next.prev.get()
		if nr, nm := node.next.get(); link1Mark || nr != next || nm { // next deleted or node moved on
			break
		}
		if next.prev.compareAndSet(link1Ref, node, false, false) {
			if _, pm := node.prev.get(); pm {
				d.helpInsert(node, next)
			}
			break
		}
		runtime.Gosched()
	}
}

// PopFront removes and returns the value at the left end (PopLeft).
func (d *Deque[T]) PopFront() (v T, ok bool) {
	prev := d.head
	var node *denode[T]
	for {
		node = derefActive(&prev.next)
		if node == d.tail {
			return v, false
		}
		link1Ref, link1Mark := node.next.get()
		if link1Mark { // node already logically deleted: help and retry
			d.helpDelete(node)
			continue
		}
		if node.next.compareAndSet(link1Ref, link1Ref, false, true) { // mark node deleted
			d.helpDelete(node)
			next := node.next.reference() // = link1Ref
			prev = d.helpInsert(prev, next)
			return node.value, true
		}
		runtime.Gosched()
	}
}

// PopBack removes and returns the value at the right end (PopRight).
func (d *Deque[T]) PopBack() (v T, ok bool) {
	next := d.tail
	node := derefActive(&next.prev)
	for {
		if r, mark := node.next.get(); r != next || mark { // node.next != (next,F)
			node = d.helpInsert(node, next)
			continue
		}
		if node == d.head {
			return v, false
		}
		if node.next.compareAndSet(next, next, false, true) { // mark node deleted
			d.helpDelete(node)
			prev := node.prev.reference()
			d.helpInsert(prev, next)
			return node.value, true
		}
		runtime.Gosched()
	}
}

// markPrev sets the deletion mark on node's prev link (MarkPrev).
func (d *Deque[T]) markPrev(node *denode[T]) {
	for {
		link1Ref, link1Mark := node.prev.get()
		if link1Mark || node.prev.compareAndSet(link1Ref, link1Ref, false, true) {
			break
		}
	}
}

// helpDelete splices a logically-deleted node out of the list (HelpDelete).
func (d *Deque[T]) helpDelete(node *denode[T]) {
	d.markPrev(node)
	var last *denode[T]
	prev := node.prev.reference()
	next := node.next.reference()
	for {
		if prev == next { // fully unlinked
			break
		}
		if _, nnm := next.next.get(); nnm { // next is also deleted: advance
			d.markPrev(next)
			next = next.next.reference()
			continue
		}
		prev2 := derefActive(&prev.next)
		if prev2 == nil { // prev is deleted
			if last != nil {
				d.markPrev(prev)
				next2 := prev.next.reference()
				last.next.compareAndSet(prev, next2, false, false)
				prev = last
				last = nil
			} else {
				prev = prev.prev.reference()
			}
			continue
		}
		if prev2 != node { // prev no longer immediately precedes node
			last = prev
			prev = prev2
			continue
		}
		if prev.next.compareAndSet(node, next, false, false) { // unlink node
			break
		}
		runtime.Gosched()
	}
}

// helpInsert corrects prev pointers so that node's predecessor is valid, and
// returns that predecessor (HelpInsert).
func (d *Deque[T]) helpInsert(prev, node *denode[T]) *denode[T] {
	var last *denode[T]
	for {
		prev2 := derefActive(&prev.next)
		if prev2 == nil { // prev is deleted
			if last != nil {
				d.markPrev(prev)
				next2 := prev.next.reference()
				last.next.compareAndSet(prev, next2, false, false)
				prev = last
				last = nil
			} else {
				prev = prev.prev.reference()
			}
			continue
		}
		link1Ref, link1Mark := node.prev.get()
		if link1Mark { // node deleted: nothing to correct
			break
		}
		if prev2 != node {
			last = prev
			prev = prev2
			continue
		}
		if link1Ref == prev { // prev pointer already correct
			break
		}
		if pnr, _ := prev.next.get(); pnr == node &&
			node.prev.compareAndSet(link1Ref, prev, false, false) {
			if _, ppm := prev.prev.get(); ppm {
				continue
			}
			break
		}
		runtime.Gosched()
	}
	return prev
}
