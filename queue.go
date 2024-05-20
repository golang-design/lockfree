// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// Queue implements lock-free FIFO freelist based queue.
// ref: https://dl.acm.org/citation.cfm?doid=248052.248106
type Queue struct {
	head unsafe.Pointer
	tail unsafe.Pointer
	len  uint64
	pool sync.Pool
}

// NewQueue creates a new lock-free queue.
func NewQueue() *Queue {
	head := directItem{next: nil, v: nil} // allocate a free item
	return &Queue{
		tail: unsafe.Pointer(&head), // both head and tail points
		head: unsafe.Pointer(&head), // to the free item
		pool: sync.Pool{
			New: func() interface{} {
				return &directItem{}
			},
		},
	}
}

// Enqueue puts the given value v at the tail of the queue.
func (q *Queue) Enqueue(v interface{}) {
	i := q.pool.Get().(*directItem)
	i.next = nil
	i.v = v

	var last, lastnext *directItem
	for {
		last = loaditem(&q.tail)
		lastnext = loaditem(&last.next)
		if loaditem(&q.tail) == last { // are tail and next consistent?
			if lastnext == nil { // was tail pointing to the last node?
				if casitem(&last.next, lastnext, i) { // try to link item at the end of linked list
					casitem(&q.tail, last, i) // enqueue is done. try swing tail to the inserted node
					atomic.AddUint64(&q.len, 1)
					return
				}
			} else { // tail was not pointing to the last node
				casitem(&q.tail, last, lastnext) // try swing tail to the next node
			}
		}
	}
}

// Dequeue removes and returns the value at the head of the queue.
// It returns nil if the queue is empty.
func (q *Queue) Dequeue() interface{} {
	var first, last, firstnext *directItem
	for {
		first = loaditem(&q.head)
		last = loaditem(&q.tail)
		firstnext = loaditem(&first.next)
		if first == loaditem(&q.head) { // are head, tail and next consistent?
			if first == last { // is queue empty?
				if firstnext == nil { // queue is empty, couldn't dequeue
					return nil
				}
				casitem(&q.tail, last, firstnext) // tail is falling behind, try to advance it
			} else { // read value before cas, otherwise another dequeue might free the next node
				v := firstnext.v
				if casitem(&q.head, first, firstnext) { // try to swing head to the next node
					atomic.AddUint64(&q.len, ^uint64(0))
					q.pool.Put(first)
					return v // queue was not empty and dequeue finished.
				}
			}
		}
	}
}

// Length returns the length of the queue.
func (q *Queue) Length() uint64 {
	return atomic.LoadUint64(&q.len)
}
