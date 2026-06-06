// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package conformtest

import (
	"math/rand/v2"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree"
)

// StackFactory builds a stack supporting up to maxParticipants concurrent
// participants and returns a function that yields one participant's view of the
// stack. The yielded function is called once per goroutine: a lock-free stack
// (unbounded participants) returns the same value every time, while a wait-free
// stack returns a fresh per-goroutine handle.
type StackFactory func(maxParticipants int) (participant func() lockfree.Stack[int])

// Stack runs the full behavioral conformance suite for a LIFO stack against the
// given implementation.
func Stack(t *testing.T, factory StackFactory) {
	t.Helper()
	t.Run("Sequential", func(t *testing.T) { stackSequential(t, factory) })
	t.Run("Differential", func(t *testing.T) { stackDifferential(t, factory) })
	t.Run("ConcurrentConservation", func(t *testing.T) { stackConservation(t, factory) })
}

// stackSequential checks LIFO ordering and empty behavior with one participant.
func stackSequential(t *testing.T, factory StackFactory) {
	s := factory(1)()
	if v, ok := s.Pop(); ok {
		t.Fatalf("pop empty returned (%v, true)", v)
	}
	for i := 0; i < 100; i++ {
		s.Push(i)
	}
	for i := 99; i >= 0; i-- {
		v, ok := s.Pop()
		if !ok || v != i {
			t.Fatalf("pop: got (%v,%v), want (%d,true)", v, ok, i)
		}
	}
	if v, ok := s.Pop(); ok {
		t.Fatalf("pop drained returned (%v, true)", v)
	}
}

// stackDifferential replays a long random op sequence against a slice reference.
func stackDifferential(t *testing.T, factory StackFactory) {
	const ops = 100000
	s := factory(1)()
	rng := rand.New(rand.NewPCG(1, 2))
	var ref []int
	for i := 0; i < ops; i++ {
		if rng.IntN(2) == 0 {
			v := int(rng.Int())
			s.Push(v)
			ref = append(ref, v)
		} else {
			gv, gok := s.Pop()
			if len(ref) == 0 {
				if gok {
					t.Fatalf("pop empty: got (%v, true)", gv)
				}
				continue
			}
			wv := ref[len(ref)-1]
			ref = ref[:len(ref)-1]
			if !gok || gv != wv {
				t.Fatalf("pop: got (%v,%v), want (%v,true)", gv, gok, wv)
			}
		}
	}
}

// stackConservation runs full multi-pusher/multi-popper and asserts every value
// is popped exactly once. A stack has no cross-producer ordering invariant under
// concurrency, so this checks conservation only. Participants are oversubscribed
// vs GOMAXPROCS to force mid-operation preemption (and, for wait-free, helping).
func stackConservation(t *testing.T, factory StackFactory) {
	const pushers = 16
	const poppers = 8
	const perPusher = 4000
	const expected = pushers * perPusher
	participant := factory(pushers + poppers)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))

	var pg sync.WaitGroup
	pg.Add(pushers)
	for p := 0; p < pushers; p++ {
		go func(p int) {
			defer pg.Done()
			s := participant()
			base := p * perPusher
			for i := 0; i < perPusher; i++ {
				s.Push(base + i)
			}
		}(p)
	}

	var popped atomic.Int64
	results := make([][]int, poppers)
	var cg sync.WaitGroup
	cg.Add(poppers)
	for c := 0; c < poppers; c++ {
		go func(c int) {
			defer cg.Done()
			s := participant()
			var mine []int
			for {
				v, ok := s.Pop()
				if ok {
					mine = append(mine, v)
					popped.Add(1)
					continue
				}
				if popped.Load() >= expected {
					results[c] = mine
					return
				}
				runtime.Gosched()
			}
		}(c)
	}
	pg.Wait()
	cg.Wait()

	seen := make([]int32, expected)
	total := 0
	for _, mine := range results {
		for _, v := range mine {
			if v < 0 || v >= expected {
				t.Fatalf("popped out-of-range value %d", v)
			}
			seen[v]++
			total++
		}
	}
	if total != expected {
		t.Fatalf("conservation: popped %d, want %d", total, expected)
	}
	for v, n := range seen {
		if n != 1 {
			t.Fatalf("value %d popped %d times, want exactly 1", v, n)
		}
	}
}
