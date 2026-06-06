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

// DequeFactory builds a deque supporting up to maxParticipants concurrent
// participants and returns a function that yields one participant's view. A
// lock-free deque (unbounded participants) returns the same value every time.
type DequeFactory func(maxParticipants int) (participant func() lockfree.Deque[int])

// Deque runs the full behavioral conformance suite for a double-ended queue.
func Deque(t *testing.T, factory DequeFactory) {
	t.Helper()
	t.Run("Sequential", func(t *testing.T) { dequeSequential(t, factory) })
	t.Run("Differential", func(t *testing.T) { dequeDifferential(t, factory) })
	t.Run("ConcurrentConservation", func(t *testing.T) { dequeConservation(t, factory) })
	t.Run("NearEmpty", func(t *testing.T) { dequeNearEmpty(t, factory) })
}

// dequeSequential checks the four operations and the empty cases on one deque.
func dequeSequential(t *testing.T, factory DequeFactory) {
	d := factory(1)()
	if v, ok := d.PopFront(); ok {
		t.Fatalf("PopFront empty returned (%v, true)", v)
	}
	if v, ok := d.PopBack(); ok {
		t.Fatalf("PopBack empty returned (%v, true)", v)
	}
	// Build 0..9 left-to-right via PushBack, drain from the front: FIFO.
	for i := 0; i < 10; i++ {
		d.PushBack(i)
	}
	for i := 0; i < 10; i++ {
		if v, ok := d.PopFront(); !ok || v != i {
			t.Fatalf("PopFront: got (%v,%v), want (%d,true)", v, ok, i)
		}
	}
	// Build via PushFront, drain from the front: LIFO.
	for i := 0; i < 10; i++ {
		d.PushFront(i)
	}
	for i := 9; i >= 0; i-- {
		if v, ok := d.PopFront(); !ok || v != i {
			t.Fatalf("PopFront after PushFront: got (%v,%v), want (%d,true)", v, ok, i)
		}
	}
	// Build via PushBack, drain from the back: LIFO.
	for i := 0; i < 10; i++ {
		d.PushBack(i)
	}
	for i := 9; i >= 0; i-- {
		if v, ok := d.PopBack(); !ok || v != i {
			t.Fatalf("PopBack: got (%v,%v), want (%d,true)", v, ok, i)
		}
	}
}

// refDeque is the sequential reference implementation.
type refDeque struct{ v []int }

func (r *refDeque) PushFront(x int) { r.v = append([]int{x}, r.v...) }
func (r *refDeque) PushBack(x int)  { r.v = append(r.v, x) }
func (r *refDeque) PopFront() (int, bool) {
	if len(r.v) == 0 {
		return 0, false
	}
	x := r.v[0]
	r.v = r.v[1:]
	return x, true
}
func (r *refDeque) PopBack() (int, bool) {
	if len(r.v) == 0 {
		return 0, false
	}
	x := r.v[len(r.v)-1]
	r.v = r.v[:len(r.v)-1]
	return x, true
}

// dequeDifferential replays a long random op sequence over all four operations
// against the reference, catching both front- and back-pointer corruption.
func dequeDifferential(t *testing.T, factory DequeFactory) {
	const ops = 200000
	d := factory(1)()
	ref := &refDeque{}
	rng := rand.New(rand.NewPCG(1, 2))

	for i := 0; i < ops; i++ {
		switch rng.IntN(4) {
		case 0:
			v := int(rng.Int())
			d.PushFront(v)
			ref.PushFront(v)
		case 1:
			v := int(rng.Int())
			d.PushBack(v)
			ref.PushBack(v)
		case 2:
			gv, gok := d.PopFront()
			wv, wok := ref.PopFront()
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("op %d PopFront: got (%v,%v), want (%v,%v)", i, gv, gok, wv, wok)
			}
		case 3:
			gv, gok := d.PopBack()
			wv, wok := ref.PopBack()
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("op %d PopBack: got (%v,%v), want (%v,%v)", i, gv, gok, wv, wok)
			}
		}
	}
	// Drain both ends and compare.
	for {
		gv, gok := d.PopFront()
		wv, wok := ref.PopFront()
		if gok != wok || (wok && gv != wv) {
			t.Fatalf("drain PopFront: got (%v,%v), want (%v,%v)", gv, gok, wv, wok)
		}
		if !gok {
			break
		}
	}
}

// dequeNearEmpty stresses the size 0<->1 boundary so PopFront and PopBack race
// for the same last node. Each worker strictly alternates push and pop (random
// ends), so it holds at most one outstanding element and the combined size stays
// bounded by the worker count and keeps returning to empty. A free 50/50 mix
// would instead drift away from empty (a reflected symmetric random walk grows
// like the square root of the op count), defeating the point.
//
// Two invariants are checked. Conservation: a live counter (pushes issued minus
// pops completed) is bumped up before every push and down after every successful
// pop; it must never go negative (a phantom or double-pop would drive it below
// zero) and the deque must drain to exactly empty with each pushed value seen
// once. Near-empty: the live counter never exceeds the worker count, confirming
// the test actually concentrates on the small-size regime it claims to.
func dequeNearEmpty(t *testing.T, factory DequeFactory) {
	const workers = 16
	const budget = 5000
	participant := factory(workers + 1)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))

	var live atomic.Int64
	var maxLive atomic.Int64
	bumpLive := func() {
		n := live.Add(1)
		for {
			m := maxLive.Load()
			if n <= m || maxLive.CompareAndSwap(m, n) {
				break
			}
		}
	}
	results := make([][]int, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for g := 0; g < workers; g++ {
		go func(g int) {
			defer wg.Done()
			d := participant()
			rng := rand.New(rand.NewPCG(uint64(g)+1, 0x9e3779b9))
			base := g * budget
			pushed := 0
			var mine []int
			for i := 0; i < budget; i++ {
				if i%2 == 0 { // push turn
					bumpLive()
					if rng.IntN(2) == 0 {
						d.PushFront(base + pushed)
					} else {
						d.PushBack(base + pushed)
					}
					pushed++
					continue
				}
				// pop turn
				var v int
				var ok bool
				if rng.IntN(2) == 0 {
					v, ok = d.PopFront()
				} else {
					v, ok = d.PopBack()
				}
				if ok {
					if n := live.Add(-1); n < 0 {
						t.Errorf("live counter went negative (%d) after pop", n)
						return
					}
					mine = append(mine, v)
				}
			}
			results[g] = mine
		}(g)
	}
	wg.Wait()
	// Strict alternation lets each worker carry at most one outstanding element,
	// so the deque size never exceeds the worker count: this confirms the test
	// stayed in the small-size regime instead of drifting away from empty.
	if m := maxLive.Load(); m > workers {
		t.Fatalf("max live size %d exceeded worker count %d; test not near-empty", m, workers)
	}

	// Drain whatever is left; alternate ends to exercise both drain paths.
	d := participant()
	var drained []int
	for i := 0; ; i++ {
		var v int
		var ok bool
		if i%2 == 0 {
			v, ok = d.PopFront()
		} else {
			v, ok = d.PopBack()
		}
		if !ok {
			break
		}
		if n := live.Add(-1); n < 0 {
			t.Fatalf("live counter went negative (%d) during drain", n)
		}
		drained = append(drained, v)
	}
	if n := live.Load(); n != 0 {
		t.Fatalf("after drain live counter = %d, want 0 (deque not empty)", n)
	}

	// Every pushed value must appear exactly once across pops and the drain.
	seen := make(map[int]int)
	for _, mine := range results {
		for _, v := range mine {
			seen[v]++
		}
	}
	for _, v := range drained {
		seen[v]++
	}
	for v, n := range seen {
		if n != 1 {
			t.Fatalf("value %d seen %d times, want exactly 1", v, n)
		}
	}
}

// dequeConservation runs concurrent pushers (split across both ends) and poppers
// (split across both ends) and asserts every value is popped exactly once.
func dequeConservation(t *testing.T, factory DequeFactory) {
	const pushers = 12
	const poppers = 12
	const perPusher = 4000
	const expected = pushers * perPusher
	participant := factory(pushers + poppers)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))

	var pg sync.WaitGroup
	pg.Add(pushers)
	for p := 0; p < pushers; p++ {
		go func(p int) {
			defer pg.Done()
			d := participant()
			base := p * perPusher
			for i := 0; i < perPusher; i++ {
				if (p+i)%2 == 0 {
					d.PushFront(base + i)
				} else {
					d.PushBack(base + i)
				}
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
			d := participant()
			var mine []int
			for {
				var v int
				var ok bool
				if c%2 == 0 {
					v, ok = d.PopFront()
				} else {
					v, ok = d.PopBack()
				}
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
