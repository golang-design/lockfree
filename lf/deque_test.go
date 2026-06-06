// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree"
	"golang.design/x/lockfree/internal/conformtest"
	"golang.design/x/lockfree/lf"
)

// TestDequeConformance runs the shared deque conformance suite (sequential over
// all four operations, a both-ends differential fuzz, concurrent conservation,
// and a near-empty stress that races both ends for the same node) against the
// lock-free Sundell & Tsigas deque.
func TestDequeConformance(t *testing.T) {
	conformtest.Deque(t, func(maxParticipants int) func() lockfree.Deque[int] {
		d := lf.NewDeque[int]()
		return func() lockfree.Deque[int] { return d }
	})
}

// mutexDeque is a mutex-guarded slice deque used as the benchmark baseline.
type mutexDeque struct {
	v  []int
	mu sync.Mutex
}

func (d *mutexDeque) PushFront(x int) {
	d.mu.Lock()
	d.v = append([]int{x}, d.v...)
	d.mu.Unlock()
}

func (d *mutexDeque) PushBack(x int) {
	d.mu.Lock()
	d.v = append(d.v, x)
	d.mu.Unlock()
}

func (d *mutexDeque) PopFront() (int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.v) == 0 {
		return 0, false
	}
	x := d.v[0]
	d.v = d.v[1:]
	return x, true
}

func (d *mutexDeque) PopBack() (int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.v) == 0 {
		return 0, false
	}
	x := d.v[len(d.v)-1]
	d.v = d.v[:len(d.v)-1]
	return x, true
}

// BenchmarkDeque compares the lock-free deque against a mutex-guarded deque
// under contention, cycling all four operations across goroutines on both ends.
func BenchmarkDeque(b *testing.B) {
	impls := []struct {
		name string
		d    lockfree.Deque[int]
	}{
		{"lockfree", lf.NewDeque[int]()},
		{"mutex", &mutexDeque{}},
	}
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			var c int64
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					switch atomic.AddInt64(&c, 1) & 3 {
					case 0:
						impl.d.PushFront(1)
					case 1:
						impl.d.PushBack(1)
					case 2:
						impl.d.PopFront()
					default:
						impl.d.PopBack()
					}
				}
			})
		})
	}
}
