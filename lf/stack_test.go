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

func TestStackPopEmpty(t *testing.T) {
	s := lf.NewStack[int]()
	if v, ok := s.Pop(); ok {
		t.Fatalf("pop empty stack returns ok, got %v", v)
	}
	if got := s.Length(); got != 0 {
		t.Fatalf("empty stack length: got %d, want 0", got)
	}
}

func TestStackPushPop(t *testing.T) {
	s := lf.NewStack[int]()
	for i := 0; i < 100; i++ {
		s.Push(i)
	}
	if got := s.Length(); got != 100 {
		t.Fatalf("length after 100 pushes: got %d, want 100", got)
	}
	for i := 99; i >= 0; i-- {
		v, ok := s.Pop()
		if !ok || v != i {
			t.Fatalf("pop: got (%v, %v), want (%d, true)", v, ok, i)
		}
	}
	if got := s.Length(); got != 0 {
		t.Fatalf("length after draining: got %d, want 0", got)
	}
}

func ExampleStack() {
	s := lf.NewStack[int]()

	s.Push(1)
	s.Push(2)
	s.Push(3)

	for i := 0; i < 3; i++ {
		v, _ := s.Pop()
		fmt.Println(v)
	}

	// Output:
	// 3
	// 2
	// 1
}

// TestStackConcurrent stresses the stack with concurrent producers and
// consumers and asserts item conservation: every value pushed is popped
// exactly once, none lost or duplicated. Run with -race to also check memory
// safety. (Note: -race proves safety, not lock-freedom; see the doc comment on
// Stack for the progress-guarantee argument.)
func TestStackConcurrent(t *testing.T) {
	const producers = 8
	const perProducer = 10000
	s := lf.NewStack[int]()

	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				s.Push(1)
			}
		}()
	}

	var popped int64
	var cg sync.WaitGroup
	cg.Add(producers)
	done := make(chan struct{})
	for c := 0; c < producers; c++ {
		go func() {
			defer cg.Done()
			for {
				if _, ok := s.Pop(); ok {
					atomic.AddInt64(&popped, 1)
					continue
				}
				select {
				case <-done:
					if _, ok := s.Pop(); ok {
						atomic.AddInt64(&popped, 1)
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

	if want := int64(producers * perProducer); popped != want {
		t.Fatalf("conservation violated: popped %d, want %d", popped, want)
	}
	if got := s.Length(); got != 0 {
		t.Fatalf("length after drain: got %d, want 0", got)
	}
}

type mutexStack struct {
	v  []int
	mu sync.Mutex
}

func newMutexStack() *mutexStack {
	return &mutexStack{v: make([]int, 0)}
}

func (s *mutexStack) Push(v int) {
	s.mu.Lock()
	s.v = append(s.v, v)
	s.mu.Unlock()
}

func (s *mutexStack) Pop() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.v) == 0 {
		return 0, false
	}
	v := s.v[len(s.v)-1]
	s.v = s.v[:len(s.v)-1]
	return v, true
}

// BenchmarkStack compares the four stack implementations (mutex, lock-free
// Treiber, lock-free elimination-backoff, wait-free) under a balanced push/pop
// workload swept across goroutine counts. The stack is prefilled so pops mostly
// succeed instead of hitting the cheap empty path.
//
// The wait-free stack does more work per operation (it scans the participant
// state and helps stalled peers), so it is slower on this throughput measure at
// every level. That is the expected trade: its value is a bounded worst-case
// latency per operation under adversarial scheduling, which an averaged
// throughput number cannot show.
func BenchmarkStack(b *testing.B) {
	const prefill = 1024
	impls := []struct {
		name  string
		build func(maxParticipants int) func(pb *testing.PB)
	}{
		{"mutex", func(int) func(*testing.PB) {
			s := newMutexStack()
			for i := 0; i < prefill; i++ {
				s.Push(i)
			}
			return balancedPushPop(func() (func(), func()) {
				return func() { s.Push(1) }, func() { s.Pop() }
			})
		}},
		{"lockfree", func(int) func(*testing.PB) {
			s := lf.NewStack[int]()
			for i := 0; i < prefill; i++ {
				s.Push(i)
			}
			return balancedPushPop(func() (func(), func()) {
				return func() { s.Push(1) }, func() { s.Pop() }
			})
		}},
		{"elimination", func(int) func(*testing.PB) {
			s := lf.NewEliminationStack[int]()
			for i := 0; i < prefill; i++ {
				s.Push(i)
			}
			return balancedPushPop(func() (func(), func()) {
				return func() { s.Push(1) }, func() { s.Pop() }
			})
		}},
		{"waitfree", func(maxParticipants int) func(*testing.PB) {
			s := wf.NewStack[int](maxParticipants)
			pre := s.Handle()
			for i := 0; i < prefill; i++ {
				pre.Push(i)
			}
			return balancedPushPop(func() (func(), func()) {
				h := s.Handle()
				return func() { h.Push(1) }, func() { h.Pop() }
			})
		}},
	}
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) { runSweep(b, impl.build) })
	}
}
