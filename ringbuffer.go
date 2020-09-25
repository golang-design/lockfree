// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree

// RingBuffer implements ring buffer queue
type RingBuffer struct {
	buf        []interface{}
	head, tail int
	len, cap   int
}

// NewRingBuffer creates a ring buffer with given capacity
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buf: make([]interface{}, capacity),
		cap: capacity,
	}
}

// Put puts x into ring buffer
func (rb *RingBuffer) Put(x interface{}) (ok bool) {
	if rb.len == rb.cap {
		return
	}

	rb.buf[rb.tail] = x
	rb.tail++
	if rb.tail > rb.cap-1 {
		rb.tail = 0
	}
	rb.len++
	ok = true
	return
}

// Get gets the first element from queue
func (rb *RingBuffer) Get() (x interface{}) {
	x = rb.buf[rb.head]
	rb.head++
	if rb.head > rb.cap-1 {
		rb.head = 0
	}
	rb.len--
	return
}

// IsFull checks if the ring buffer is full
func (rb *RingBuffer) IsFull() bool {
	return rb.len == rb.cap
}

// LookAll reads all elements from ring buffer
// this method doesn't consume all elements
func (rb *RingBuffer) LookAll() []interface{} {
	all := make([]interface{}, rb.len)
	j := 0
	for i := rb.head; ; i++ {
		if i > rb.cap-1 {
			i = 0
		}
		if i == rb.tail && j > 0 {
			break
		}
		all[j] = rb.buf[i]
		j++
	}
	return all
}
