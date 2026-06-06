// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"container/list"
	"sync"
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

// mutexDeque is a mutex-guarded doubly linked list used as the benchmark
// baseline. container/list gives O(1) operations at both ends, so the lock is
// the only thing being compared (a naive slice deque would prepend in O(n) and
// unfairly flatter the lock-free version once prefilled).
type mutexDeque struct {
	l  list.List
	mu sync.Mutex
}

func (d *mutexDeque) PushFront(x int) {
	d.mu.Lock()
	d.l.PushFront(x)
	d.mu.Unlock()
}

func (d *mutexDeque) PushBack(x int) {
	d.mu.Lock()
	d.l.PushBack(x)
	d.mu.Unlock()
}

func (d *mutexDeque) PopFront() (int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e := d.l.Front()
	if e == nil {
		return 0, false
	}
	d.l.Remove(e)
	return e.Value.(int), true
}

func (d *mutexDeque) PopBack() (int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e := d.l.Back()
	if e == nil {
		return 0, false
	}
	d.l.Remove(e)
	return e.Value.(int), true
}

// BenchmarkDeque compares the lock-free deque against a mutex-guarded deque
// under a balanced workload swept across goroutine counts. Each goroutine cycles
// PushFront, PushBack, PopFront, PopBack with a local counter (no shared
// op-selector), and the deque is prefilled so pops mostly succeed.
func BenchmarkDeque(b *testing.B) {
	const prefill = 1024
	impls := []struct {
		name string
		make func() lockfree.Deque[int]
	}{
		{"mutex", func() lockfree.Deque[int] { return &mutexDeque{} }},
		{"lockfree", func() lockfree.Deque[int] { return lf.NewDeque[int]() }},
	}
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			runSweep(b, func(int) func(*testing.PB) {
				d := impl.make()
				for i := 0; i < prefill; i++ {
					d.PushBack(i)
				}
				return func(pb *testing.PB) {
					var n uint
					for pb.Next() {
						switch n & 3 {
						case 0:
							d.PushFront(1)
						case 1:
							d.PushBack(1)
						case 2:
							d.PopFront()
						default:
							d.PopBack()
						}
						n++
					}
				}
			})
		})
	}
}
