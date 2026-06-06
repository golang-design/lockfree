// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree/lf"
	"golang.design/x/lockfree/wf"
)

func TestQueueDequeueEmpty(t *testing.T) {
	q := lf.NewQueue[int]()
	if v, ok := q.Dequeue(); ok {
		t.Fatalf("dequeue empty queue returns ok, got %v", v)
	}
}

func TestQueue_Length(t *testing.T) {
	q := lf.NewQueue[int]()
	if q.Length() != 0 {
		t.Fatalf("empty queue has non-zero length")
	}

	q.Enqueue(1)
	if q.Length() != 1 {
		t.Fatalf("count of enqueue wrong, want %d, got %d.", 1, q.Length())
	}

	if v, ok := q.Dequeue(); !ok || v != 1 {
		t.Fatalf("dequeue: got (%v, %v), want (1, true)", v, ok)
	}
	if q.Length() != 0 {
		t.Fatalf("count of dequeue wrong, want %d, got %d", 0, q.Length())
	}
}

func TestQueueFIFO(t *testing.T) {
	q := lf.NewQueue[int]()
	for i := 0; i < 100; i++ {
		q.Enqueue(i)
	}
	for i := 0; i < 100; i++ {
		v, ok := q.Dequeue()
		if !ok || v != i {
			t.Fatalf("dequeue: got (%v, %v), want (%d, true)", v, ok, i)
		}
	}
}

func ExampleQueue() {
	q := lf.NewQueue[string]()

	q.Enqueue("1st item")
	q.Enqueue("2nd item")
	q.Enqueue("3rd item")

	for i := 0; i < 3; i++ {
		v, _ := q.Dequeue()
		fmt.Println(v)
	}

	// Output:
	// 1st item
	// 2nd item
	// 3rd item
}

// TestQueueConcurrent stresses the queue with concurrent producers and
// consumers and asserts item conservation. Run with -race for memory safety.
func TestQueueConcurrent(t *testing.T) {
	const producers = 8
	const perProducer = 10000
	q := lf.NewQueue[int]()

	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				q.Enqueue(1)
			}
		}()
	}

	var dequeued int64
	var cg sync.WaitGroup
	cg.Add(producers)
	done := make(chan struct{})
	for c := 0; c < producers; c++ {
		go func() {
			defer cg.Done()
			for {
				if _, ok := q.Dequeue(); ok {
					atomic.AddInt64(&dequeued, 1)
					continue
				}
				select {
				case <-done:
					if _, ok := q.Dequeue(); ok {
						atomic.AddInt64(&dequeued, 1)
						continue
					}
					return
				default:
				}
			}
		}()
	}

	wg.Wait()
	close(done)
	cg.Wait()

	if want := int64(producers * perProducer); dequeued != want {
		t.Fatalf("conservation violated: dequeued %d, want %d", dequeued, want)
	}
	if got := q.Length(); got != 0 {
		t.Fatalf("length after drain: got %d, want 0", got)
	}
}

// queueInterface lets the benchmark drive both implementations identically.
type queueInterface interface {
	Enqueue(int)
	Dequeue() (int, bool)
}

type mutexQueue struct {
	v  []int
	mu sync.Mutex
}

func newMutexQueue() *mutexQueue {
	return &mutexQueue{v: make([]int, 0)}
}

func (q *mutexQueue) Enqueue(v int) {
	q.mu.Lock()
	q.v = append(q.v, v)
	q.mu.Unlock()
}

func (q *mutexQueue) Dequeue() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.v) == 0 {
		return 0, false
	}
	v := q.v[0]
	q.v = q.v[1:]
	return v, true
}

// BenchmarkQueue compares the three queue implementations (mutex, lock-free
// Michael & Scott, wait-free Kogan & Petrank) under a balanced enqueue/dequeue
// workload swept across goroutine counts. The queue is prefilled so dequeues
// mostly succeed instead of hitting the cheap empty path.
//
// As with the stack, the wait-free queue is slower on this throughput measure
// because it scans participant state and helps stalled peers; it buys a bounded
// per-operation latency that an averaged throughput number does not show.
func BenchmarkQueue(b *testing.B) {
	const prefill = 1024
	impls := []struct {
		name  string
		build func(maxParticipants int) func(pb *testing.PB)
	}{
		{"mutex", func(int) func(*testing.PB) {
			q := newMutexQueue()
			for i := 0; i < prefill; i++ {
				q.Enqueue(i)
			}
			return balancedPushPop(func() (func(), func()) {
				return func() { q.Enqueue(1) }, func() { q.Dequeue() }
			})
		}},
		{"lockfree", func(int) func(*testing.PB) {
			q := lf.NewQueue[int]()
			for i := 0; i < prefill; i++ {
				q.Enqueue(i)
			}
			return balancedPushPop(func() (func(), func()) {
				return func() { q.Enqueue(1) }, func() { q.Dequeue() }
			})
		}},
		{"waitfree", func(maxParticipants int) func(*testing.PB) {
			q := wf.NewQueue[int](maxParticipants)
			pre := q.Handle()
			for i := 0; i < prefill; i++ {
				pre.Enqueue(i)
			}
			return balancedPushPop(func() (func(), func()) {
				h := q.Handle()
				return func() { h.Enqueue(1) }, func() { h.Dequeue() }
			})
		}},
	}
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) { runSweep(b, impl.build) })
	}
}
