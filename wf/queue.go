// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

// Package wf provides concurrent data structures with a wait-free progress
// guarantee: every operation completes in a bounded number of its own steps,
// regardless of how the scheduler interleaves goroutines. Wait-freedom is
// strictly stronger than lock-freedom (it additionally rules out starvation),
// at the cost of a higher constant factor.
package wf

import "sync/atomic"

// noTid marks a node that has not yet been claimed by a dequeuer.
const noTid = -1

// node is a queue node. value and enqTid are immutable after construction and
// become visible to other goroutines through the atomic CAS that links the node
// into the list, so they need no further synchronization. next and deqTid are
// mutated concurrently and are atomic.
type node[T any] struct {
	value  T
	next   atomic.Pointer[node[T]]
	enqTid int
	deqTid atomic.Int64
}

func newNode[T any](value T, enqTid int) *node[T] {
	n := &node[T]{value: value, enqTid: enqTid}
	n.deqTid.Store(noTid)
	return n
}

// opDesc is an operation descriptor published in the announce array. It is
// immutable: progress is made by CAS-replacing a slot's descriptor with a new
// one, never by mutating fields in place.
type opDesc[T any] struct {
	phase   int64
	pending bool
	enqueue bool
	node    *node[T]
}

// Queue is a wait-free FIFO queue for multiple enqueuers and dequeuers
// (Kogan & Petrank, "Wait-Free Queues With Multiple Enqueuers and Dequeuers",
// PPoPP 2011).
//
// Progress guarantee: wait-free. Before doing its own work, every operation
// announces itself (with a monotonically increasing phase) in a per-participant
// slot, and every other operation helps all announced operations with a phase no
// greater than its own complete before returning. An operation therefore cannot
// be starved: once announced, it is finished within a bounded number of steps
// (O(maxHandles) per operation) even if its own goroutine is never scheduled
// again. There are no locks and no operation waits on another's completion.
//
// Memory reclamation relies on Go's garbage collector, which keeps a node alive
// while any goroutine still references it; this also avoids the ABA hazard
// without hazard pointers.
//
// Participants must register: because the helping mechanism is indexed by a
// participant id and Go has no goroutine id, callers obtain a Handle (one per
// goroutine) up to the maxHandles bound declared at construction. Each operation
// is O(maxHandles), the documented cost of the wait-free latency bound.
type Queue[T any] struct {
	head  atomic.Pointer[node[T]]
	tail  atomic.Pointer[node[T]]
	state []atomic.Pointer[opDesc[T]] // announce array, one slot per handle
	next  atomic.Int64                // handle allocation cursor
	max   int
}

// NewQueue creates a wait-free queue that supports up to maxHandles concurrent
// participants. maxHandles must be at least 1.
func NewQueue[T any](maxHandles int) *Queue[T] {
	if maxHandles < 1 {
		panic("wf: NewQueue maxHandles must be >= 1")
	}
	var zero T
	sentinel := newNode(zero, noTid)
	q := &Queue[T]{
		state: make([]atomic.Pointer[opDesc[T]], maxHandles),
		max:   maxHandles,
	}
	q.head.Store(sentinel)
	q.tail.Store(sentinel)
	for i := range q.state {
		q.state[i].Store(&opDesc[T]{phase: -1, pending: false, enqueue: true, node: nil})
	}
	return q
}

// Handle is a participant's access point to a Queue. It is bound to one slot in
// the announce array and is NOT safe for concurrent use by multiple goroutines:
// acquire one Handle per goroutine. The number of handles may not exceed the
// maxHandles passed to NewQueue.
type Handle[T any] struct {
	q   *Queue[T]
	tid int
}

// Handle registers a new participant and returns its Handle. It panics if more
// than maxHandles handles are requested.
func (q *Queue[T]) Handle() *Handle[T] {
	id := q.next.Add(1) - 1
	if id >= int64(q.max) {
		panic("wf: too many handles (exceeds maxHandles passed to NewQueue)")
	}
	return &Handle[T]{q: q, tid: int(id)}
}

// Enqueue appends v to the tail of the queue.
func (h *Handle[T]) Enqueue(v T) { h.q.enq(h.tid, v) }

// Dequeue removes and returns the value at the head of the queue. The second
// return value is false if the queue was empty during the operation.
func (h *Handle[T]) Dequeue() (T, bool) { return h.q.deq(h.tid) }

// maxPhase returns the largest phase currently announced, or -1 if none.
func (q *Queue[T]) maxPhase() int64 {
	maxP := int64(-1)
	for i := range q.state {
		if p := q.state[i].Load().phase; p > maxP {
			maxP = p
		}
	}
	return maxP
}

func (q *Queue[T]) isStillPending(tid int, phase int64) bool {
	d := q.state[tid].Load()
	return d.pending && d.phase <= phase
}

func (q *Queue[T]) enq(tid int, value T) {
	phase := q.maxPhase() + 1
	q.state[tid].Store(&opDesc[T]{phase: phase, pending: true, enqueue: true, node: newNode(value, tid)})
	q.help(phase)
	q.helpFinishEnq()
}

func (q *Queue[T]) deq(tid int) (T, bool) {
	phase := q.maxPhase() + 1
	q.state[tid].Store(&opDesc[T]{phase: phase, pending: true, enqueue: false, node: nil})
	q.help(phase)
	q.helpFinishDeq()
	node := q.state[tid].Load().node
	if node == nil { // queue was empty during the operation
		var zero T
		return zero, false
	}
	return node.next.Load().value, true
}

// help completes every announced operation whose phase is no greater than the
// caller's phase, which is what makes the queue wait-free.
func (q *Queue[T]) help(phase int64) {
	for i := range q.state {
		desc := q.state[i].Load()
		if desc.pending && desc.phase <= phase {
			if desc.enqueue {
				q.helpEnq(i, phase)
			} else {
				q.helpDeq(i, phase)
			}
		}
	}
}

func (q *Queue[T]) helpEnq(tid int, phase int64) {
	for q.isStillPending(tid, phase) {
		last := q.tail.Load()
		next := last.next.Load()
		if last != q.tail.Load() {
			continue
		}
		if next == nil { // tail is the last node: try to link
			if !q.isStillPending(tid, phase) {
				return
			}
			if last.next.CompareAndSwap(nil, q.state[tid].Load().node) {
				q.helpFinishEnq()
				return
			}
		} else { // tail is lagging: help advance it
			q.helpFinishEnq()
		}
	}
}

func (q *Queue[T]) helpFinishEnq() {
	last := q.tail.Load()
	next := last.next.Load()
	if next == nil {
		return
	}
	tid := next.enqTid
	curDesc := q.state[tid].Load()
	if last == q.tail.Load() && q.state[tid].Load().node == next {
		newDesc := &opDesc[T]{phase: curDesc.phase, pending: false, enqueue: true, node: next}
		q.state[tid].CompareAndSwap(curDesc, newDesc)
		q.tail.CompareAndSwap(last, next)
	}
}

func (q *Queue[T]) helpDeq(tid int, phase int64) {
	for q.isStillPending(tid, phase) {
		first := q.head.Load()
		last := q.tail.Load()
		next := first.next.Load()
		if first != q.head.Load() {
			continue
		}
		if first == last { // queue is empty or tail is lagging
			if next == nil { // empty: mark the dequeue as done with no node
				curDesc := q.state[tid].Load()
				if last == q.tail.Load() && q.isStillPending(tid, phase) {
					newDesc := &opDesc[T]{phase: curDesc.phase, pending: false, enqueue: false, node: nil}
					q.state[tid].CompareAndSwap(curDesc, newDesc)
				}
			} else { // an enqueue is in progress: help it finish
				q.helpFinishEnq()
			}
			continue
		}
		// Queue is non-empty: record the candidate predecessor, then claim it.
		curDesc := q.state[tid].Load()
		n := curDesc.node
		if !q.isStillPending(tid, phase) {
			break
		}
		if first == q.head.Load() && n != first {
			newDesc := &opDesc[T]{phase: curDesc.phase, pending: true, enqueue: false, node: first}
			if !q.state[tid].CompareAndSwap(curDesc, newDesc) {
				continue
			}
		}
		first.deqTid.CompareAndSwap(noTid, int64(tid))
		q.helpFinishDeq()
	}
}

func (q *Queue[T]) helpFinishDeq() {
	first := q.head.Load()
	next := first.next.Load()
	tid := int(first.deqTid.Load())
	if tid == noTid {
		return
	}
	curDesc := q.state[tid].Load()
	if first == q.head.Load() && next != nil {
		newDesc := &opDesc[T]{phase: curDesc.phase, pending: false, enqueue: curDesc.enqueue, node: curDesc.node}
		q.state[tid].CompareAndSwap(curDesc, newDesc)
		q.head.CompareAndSwap(first, next)
	}
}
