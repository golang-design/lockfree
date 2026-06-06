// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree_test

import (
	"sync"
	"testing"

	"golang.design/x/lockfree"
)

func TestRingBuffer(t *testing.T) {
	rb := lockfree.NewRingBuffer[int](10) // rounds up to 16

	if got := rb.Cap(); got != 16 {
		t.Fatalf("Cap: got %d, want 16 (next power of two of 10)", got)
	}
	if !rb.IsEmpty() {
		t.Fatalf("fresh buffer is not empty")
	}

	for i := 0; i < 16; i++ {
		if ok := rb.Put(i); !ok {
			t.Fatalf("Put(%d) failed while buffer had room", i)
		}
	}
	if !rb.IsFull() {
		t.Fatalf("buffer should be full after 16 puts")
	}
	if got := rb.Len(); got != 16 {
		t.Fatalf("Len: got %d, want 16", got)
	}
	if ok := rb.Put(99); ok {
		t.Fatalf("Put succeeded on a full buffer")
	}

	for i := 0; i < 16; i++ {
		v, ok := rb.Get()
		if !ok || v != i {
			t.Fatalf("Get: got (%v, %v), want (%d, true)", v, ok, i)
		}
	}
	if _, ok := rb.Get(); ok {
		t.Fatalf("Get succeeded on an empty buffer")
	}
}

func TestRingBufferMinCapacity(t *testing.T) {
	rb := lockfree.NewRingBuffer[int](0) // rounds up to 1
	if got := rb.Cap(); got != 1 {
		t.Fatalf("Cap: got %d, want 1", got)
	}
	if !rb.Put(42) {
		t.Fatalf("Put failed on empty single-slot buffer")
	}
	if rb.Put(43) {
		t.Fatalf("Put succeeded on full single-slot buffer")
	}
	if v, ok := rb.Get(); !ok || v != 42 {
		t.Fatalf("Get: got (%v, %v), want (42, true)", v, ok)
	}
}

// TestRingBufferConcurrent runs one producer and one consumer (the SPSC
// contract) and asserts every value is delivered exactly once and in FIFO
// order. Run with -race for memory safety. (-race proves safety, not
// wait-freedom; see the doc comment on RingBuffer for that argument.)
func TestRingBufferConcurrent(t *testing.T) {
	const n = 1 << 20
	rb := lockfree.NewRingBuffer[int](1024)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // single producer
		defer wg.Done()
		for i := 0; i < n; i++ {
			for !rb.Put(i) { // spin until there is room
			}
		}
	}()

	for i := 0; i < n; i++ { // single consumer
		var v int
		var ok bool
		for {
			if v, ok = rb.Get(); ok {
				break
			}
		}
		if v != i {
			t.Fatalf("FIFO violated at %d: got %d", i, v)
		}
	}
	wg.Wait()

	if !rb.IsEmpty() {
		t.Fatalf("buffer not drained: len %d", rb.Len())
	}
}

// ringBufferInterface lets the benchmark drive both implementations identically.
type ringBufferInterface interface {
	Put(int) bool
	Get() (int, bool)
}

// mutexRingBuffer is a mutex-guarded ring buffer used as the benchmark baseline.
type mutexRingBuffer struct {
	mu         sync.Mutex
	buf        []int
	head, tail int
	count      int
}

func newMutexRingBuffer(capacity int) *mutexRingBuffer {
	return &mutexRingBuffer{buf: make([]int, capacity)}
}

func (rb *mutexRingBuffer) Put(x int) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.count == len(rb.buf) {
		return false
	}
	rb.buf[rb.tail] = x
	rb.tail = (rb.tail + 1) % len(rb.buf)
	rb.count++
	return true
}

func (rb *mutexRingBuffer) Get() (int, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.count == 0 {
		return 0, false
	}
	x := rb.buf[rb.head]
	rb.head = (rb.head + 1) % len(rb.buf)
	rb.count--
	return x, true
}

// BenchmarkRingBuffer compares the wait-free SPSC ring buffer against a
// mutex-guarded ring buffer, each driven by one producer and one consumer.
func BenchmarkRingBuffer(b *testing.B) {
	const capacity = 1024
	impls := []struct {
		name string
		rb   ringBufferInterface
	}{
		{"lockfree", lockfree.NewRingBuffer[int](capacity)},
		{"mutex", newMutexRingBuffer(capacity)},
	}
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				got := 0
				for got < b.N {
					if _, ok := impl.rb.Get(); ok {
						got++
					}
				}
			}()
			for i := 0; i < b.N; i++ {
				for !impl.rb.Put(i) {
				}
			}
			wg.Wait()
		})
	}
}
