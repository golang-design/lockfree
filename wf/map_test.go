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

// Ordered-map behavior (Set/Get/Del, differential, concurrent disjoint) is
// covered by the shared conformance suite (see conformance_test.go). The tests
// here cover wf-specific behavior: the bounded Handle budget and reclamation.

func less(a, b int) bool { return a < b }

func TestMapHandleExhaustion(t *testing.T) {
	m := wf.NewMap[int, int](2, less)
	m.Handle()
	m.Handle()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when exceeding maxHandles")
		}
	}()
	m.Handle() // third handle exceeds maxHandles=2
}

func TestNewMapInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for maxHandles < 1")
		}
	}()
	wf.NewMap[int, int](0, less)
}

// TestMapReclamation guards the universal construction's most dangerous and
// otherwise test-invisible failure mode: retaining the whole operation-list
// history (which conservation, race, and fuzz tests all pass while it leaks).
// It sets and immediately deletes many finalized values under churning keys,
// then forces GC and asserts most values are finalized. If the history were
// pinned, the persistent-tree versions would keep the values alive and few
// finalizers would run.
func TestMapReclamation(t *testing.T) {
	const m = 20000
	var finalized atomic.Int64

	mp := wf.NewMap[int, *int](2, less)
	h := mp.Handle()
	for i := 0; i < m; i++ {
		v := new(int)
		*v = i
		runtime.SetFinalizer(v, func(*int) { finalized.Add(1) })
		k := i % 64 // churn a small key range so the tree stays small
		h.Set(k, v)
		if _, ok := h.Del(k); !ok {
			t.Fatalf("del after set returned absent")
		}
	}

	for i := 0; i < 10 && finalized.Load() < m/2; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	got := finalized.Load()
	// Keep the map live across the GC cycles above, so the test measures whether
	// the *history* is reclaimed while the map is still in use, not whether the
	// whole map became collectible.
	runtime.KeepAlive(h)
	if got < m/2 {
		t.Fatalf("only %d/%d values finalized: operation history is being retained (leak)", got, m)
	}
}
