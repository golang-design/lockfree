// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf

import "sync/atomic"

// cacheLine is a typical CPU cache line size, used to pad the producer and
// consumer cursors onto separate lines to avoid false sharing.
const cacheLine = 64

// RingBuffer is a bounded single-producer/single-consumer (SPSC) FIFO ring
// buffer.
//
// Progress guarantee: wait-free. With exactly one producer and one consumer,
// Put and Get each complete in a bounded number of steps with no loops and no
// waiting: the producer owns the tail cursor and the consumer owns the head
// cursor, and they never contend on the same word. This is a stronger guarantee
// than lock-free.
//
// It is the caller's responsibility to use at most one producer goroutine
// (calling Put) and at most one consumer goroutine (calling Get) concurrently.
// Multiple producers or multiple consumers are NOT supported and will corrupt
// the buffer. Len, Cap, IsFull and IsEmpty are best-effort observations under
// concurrency.
type RingBuffer[T any] struct {
	buf  []T
	mask uint64
	_    [cacheLine]byte // pad to keep head and tail on separate cache lines
	head atomic.Uint64   // consumer cursor: index of the next item to Get
	_    [cacheLine]byte
	tail atomic.Uint64 // producer cursor: index of the next slot to Put
}

// NewRingBuffer creates a ring buffer that can hold up to capacity elements.
// capacity is rounded up to the next power of two (with a minimum of 1) so the
// cursors can be masked instead of taking a modulo; use Cap to read the
// effective capacity.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	c := nextPowerOfTwo(capacity)
	return &RingBuffer[T]{
		buf:  make([]T, c),
		mask: uint64(c - 1),
	}
}

// Put appends x to the tail of the buffer. It returns false if the buffer is
// full. Put must be called from a single producer goroutine.
func (rb *RingBuffer[T]) Put(x T) (ok bool) {
	tail := rb.tail.Load()
	head := rb.head.Load()
	if tail-head >= uint64(len(rb.buf)) { // full
		return false
	}
	rb.buf[tail&rb.mask] = x
	rb.tail.Store(tail + 1) // publish the write
	return true
}

// Get removes and returns the element at the head of the buffer. The second
// return value is false if the buffer is empty. Get must be called from a
// single consumer goroutine.
func (rb *RingBuffer[T]) Get() (x T, ok bool) {
	head := rb.head.Load()
	tail := rb.tail.Load()
	if head == tail { // empty
		return x, false
	}
	x = rb.buf[head&rb.mask]
	var zero T
	rb.buf[head&rb.mask] = zero // drop reference so the value can be GC'd
	rb.head.Store(head + 1)     // publish the read
	return x, true
}

// Len returns the number of elements currently buffered (best effort).
func (rb *RingBuffer[T]) Len() int {
	return int(rb.tail.Load() - rb.head.Load())
}

// Cap returns the effective capacity (a power of two).
func (rb *RingBuffer[T]) Cap() int {
	return len(rb.buf)
}

// IsFull reports whether the buffer is full (best effort).
func (rb *RingBuffer[T]) IsFull() bool {
	return rb.tail.Load()-rb.head.Load() >= uint64(len(rb.buf))
}

// IsEmpty reports whether the buffer is empty (best effort).
func (rb *RingBuffer[T]) IsEmpty() bool {
	return rb.head.Load() == rb.tail.Load()
}

// nextPowerOfTwo returns the smallest power of two >= n, with a minimum of 1.
func nextPowerOfTwo(n int) int {
	if n < 1 {
		return 1
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}
