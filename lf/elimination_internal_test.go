// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// TestExchangerMatches is a white-box test guarding the elimination mechanism's
// most insidious failure: a broken exchanger that never matches would still pass
// every behavioral and conservation test, because every operation falls back to
// the central Treiber stack. It drives one goroutine offering a value (a push)
// and one offering nil (a pop) at the same slot and asserts they actually meet
// and swap. Unlike an end-to-end contention test, this fires deterministically
// regardless of the number of hardware threads (the waiter yields), so it is a
// reliable signal that the match path is live.
func TestExchangerMatches(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(2))

	var e exchanger[int]
	var matches atomic.Int64
	const rounds = 1 << 20
	v := 7

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // push side: offers a value, matches only a pop (partner == nil)
		defer wg.Done()
		for i := 0; i < rounds && matches.Load() == 0; i++ {
			if p, ok := e.exchange(&v, 128); ok && p == nil {
				matches.Add(1)
			}
		}
	}()
	go func() { // pop side: offers nil, matches only a push (partner != nil)
		defer wg.Done()
		for i := 0; i < rounds && matches.Load() == 0; i++ {
			if p, ok := e.exchange(nil, 128); ok && p != nil {
				if *p != v {
					t.Errorf("exchanged wrong value: got %d, want %d", *p, v)
					return
				}
				matches.Add(1)
			}
		}
	}()
	wg.Wait()

	if matches.Load() == 0 {
		t.Fatal("exchanger never matched a push with a pop: the elimination mechanism is dead")
	}
}

// TestEliminationFires is a best-effort end-to-end check that the stack actually
// routes through elimination under contention. It requires real parallelism, so
// it is skipped when only one hardware thread is available rather than asserting
// a property the machine cannot exhibit.
func TestEliminationFires(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("elimination needs real parallelism; single-CPU machine")
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))

	s := newEliminationStack[int](4, 256)
	const workers = 8
	const ops = 50000

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				if id%2 == 0 {
					s.Push(i)
				} else {
					s.Pop()
				}
			}
		}(w)
	}
	wg.Wait()

	if got := s.eliminations.Load(); got == 0 {
		t.Fatal("elimination never fired end-to-end despite contention")
	} else {
		t.Logf("successful eliminations: %d", got)
	}
}
