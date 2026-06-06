// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree/lf"
	"golang.design/x/lockfree/wf"
)

// mutexQueue is the blocking baseline for the cross-implementation comparison.
type mutexQueue struct {
	mu sync.Mutex
	v  []int
}

func (q *mutexQueue) Enqueue(x int) {
	q.mu.Lock()
	q.v = append(q.v, x)
	q.mu.Unlock()
}

func (q *mutexQueue) Dequeue() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.v) == 0 {
		return 0, false
	}
	x := q.v[0]
	q.v = q.v[1:]
	return x, true
}

// BenchmarkQueue compares the three queue variants under contention, each
// goroutine alternating Enqueue and Dequeue. It makes the progress-guarantee
// tradeoff visible: the wait-free queue pays an O(participants) helping scan per
// operation in exchange for its bounded-latency guarantee, so it is expected to
// trail the lock-free queue here while still beating a mutex under load.
func BenchmarkQueue(b *testing.B) {
	procs := runtime.GOMAXPROCS(0)

	b.Run("lockfree", func(b *testing.B) {
		q := lf.NewQueue[int]()
		var c int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if atomic.AddInt64(&c, 1)&1 == 0 {
					q.Enqueue(1)
				} else {
					q.Dequeue()
				}
			}
		})
	})

	b.Run("waitfree", func(b *testing.B) {
		q := wf.NewQueue[int](2*procs + 8) // enough handles for the parallel goroutines
		var c int64
		b.RunParallel(func(pb *testing.PB) {
			h := q.Handle()
			for pb.Next() {
				if atomic.AddInt64(&c, 1)&1 == 0 {
					h.Enqueue(1)
				} else {
					h.Dequeue()
				}
			}
		})
	})

	b.Run("mutex", func(b *testing.B) {
		q := &mutexQueue{}
		var c int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if atomic.AddInt64(&c, 1)&1 == 0 {
					q.Enqueue(1)
				} else {
					q.Dequeue()
				}
			}
		})
	})
}
