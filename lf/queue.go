// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import "sync/atomic"

// Queue is a lock-free FIFO queue (Michael & Scott, PODC 1996).
// ref: https://dl.acm.org/citation.cfm?doid=248052.248106
//
// Progress guarantee: lock-free. Enqueue and Dequeue retry CAS loops that
// always reload head/tail/next before acting and help advance a lagging tail
// rather than waiting on the goroutine that left it behind, so a stalled
// operation never blocks the others.
//
// Memory reclamation relies on Go's garbage collector: a dequeued node stays
// alive as long as any goroutine still holds a reference, which prevents the
// classic ABA hazard without hazard pointers. Nodes are intentionally not
// pooled, because recycling them would defeat that protection.
type Queue[T any] struct {
	head atomic.Pointer[directItem[T]]
	tail atomic.Pointer[directItem[T]]
	len  atomic.Uint64
}

// NewQueue creates a new lock-free queue.
func NewQueue[T any]() *Queue[T] {
	sentinel := &directItem[T]{} // allocate a free (dummy) item
	q := &Queue[T]{}
	q.head.Store(sentinel) // both head and tail point
	q.tail.Store(sentinel) // to the free item
	return q
}

// Enqueue puts the given value v at the tail of the queue.
func (q *Queue[T]) Enqueue(v T) {
	i := &directItem[T]{v: v}
	for {
		last := q.tail.Load()
		lastnext := last.next.Load()
		if last == q.tail.Load() { // are tail and next consistent?
			if lastnext == nil { // was tail pointing to the last node?
				if last.next.CompareAndSwap(nil, i) { // link item at the end of the list
					q.tail.CompareAndSwap(last, i) // try to swing tail to the inserted node
					q.len.Add(1)
					return
				}
			} else { // tail was not pointing to the last node
				q.tail.CompareAndSwap(last, lastnext) // try to swing tail to the next node
			}
		}
	}
}

// Dequeue removes and returns the value at the head of the queue.
// The second return value is false if the queue is empty.
func (q *Queue[T]) Dequeue() (v T, ok bool) {
	for {
		first := q.head.Load()
		last := q.tail.Load()
		firstnext := first.next.Load()
		if first == q.head.Load() { // are head, tail and next consistent?
			if first == last { // is queue empty or tail falling behind?
				if firstnext == nil { // queue is empty, couldn't dequeue
					return v, false
				}
				q.tail.CompareAndSwap(last, firstnext) // tail is falling behind, advance it
			} else { // read value before CAS, otherwise another dequeue might reuse next
				v := firstnext.v
				if q.head.CompareAndSwap(first, firstnext) { // swing head to the next node
					q.len.Add(^uint64(0))
					return v, true
				}
			}
		}
	}
}

// Length returns the number of elements currently in the queue.
func (q *Queue[T]) Length() uint64 {
	return q.len.Load()
}
