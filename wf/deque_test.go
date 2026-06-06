// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"golang.design/x/lockfree/wf"
)

// Deque behavior (both ends, empty, differential, concurrent, near-empty) is
// covered by the shared conformance suite (see conformance_test.go). The tests
// here cover wf-specific behavior: the bounded Handle budget and memory
// reclamation.

func TestDequeHandleExhaustion(t *testing.T) {
	d := wf.NewDeque[int](2)
	d.Handle()
	d.Handle()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when exceeding maxHandles")
		}
	}()
	d.Handle() // third handle exceeds maxHandles=2
}

func TestNewDequeInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for maxHandles < 1")
		}
	}()
	wf.NewDeque[int](0)
}

// TestDequeReclamation guards the universal construction's most dangerous and
// otherwise test-invisible failure mode: retaining the whole operation-list
// history (which conservation, race, and fuzz tests all pass while it leaks).
// It pushes and immediately pops many finalized values, then forces GC and
// asserts most values are finalized. If the history were pinned (for example by
// a fixed sentinel reference or non-advancing cursors), the persistent-tree
// versions would keep the values alive and few finalizers would run. It mixes
// both ends so the tree-rebalancing paths run while the history is reclaimed.
func TestDequeReclamation(t *testing.T) {
	const m = 20000
	var finalized atomic.Int64

	d := wf.NewDeque[*int](2)
	h := d.Handle()
	for i := 0; i < m; i++ {
		v := new(int)
		*v = i
		runtime.SetFinalizer(v, func(*int) { finalized.Add(1) })
		if i%2 == 0 {
			h.PushBack(v)
			if _, ok := h.PopFront(); !ok {
				t.Fatalf("pop after push returned empty")
			}
		} else {
			h.PushFront(v)
			if _, ok := h.PopBack(); !ok {
				t.Fatalf("pop after push returned empty")
			}
		}
	}

	for i := 0; i < 10 && finalized.Load() < m/2; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	got := finalized.Load()
	// Keep the deque live across the GC cycles above, so the test measures
	// whether the *history* is reclaimed while the deque is still in use, not
	// whether the whole deque became collectible.
	runtime.KeepAlive(h)
	if got < m/2 {
		t.Fatalf("only %d/%d values finalized: operation history is being retained (leak)", got, m)
	}
}
