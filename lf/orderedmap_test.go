// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"math/rand/v2"
	"sync"
	"testing"

	"golang.design/x/lockfree/lf"
)

func newOrderedMap() *lf.OrderedMap[int, int] {
	return lf.NewOrderedMap[int, int](func(a, b int) bool { return a < b })
}

func TestOrderedMap_PutGetDel(t *testing.T) {
	m := newOrderedMap()
	if m.Len() != 0 {
		t.Fatalf("fresh map Len: got %d, want 0", m.Len())
	}
	if _, ok := m.Get(10); ok {
		t.Fatalf("Get on empty map returned ok")
	}
	if _, ok := m.Del(10); ok {
		t.Fatalf("Del on empty map returned ok")
	}

	for i := 0; i <= 5; i++ {
		m.Set(i, i*100)
	}
	if m.Len() != 6 {
		t.Fatalf("Len after 6 Puts: got %d, want 6", m.Len())
	}
	if v, ok := m.Get(3); !ok || v != 300 {
		t.Fatalf("Get(3): got (%v,%v), want (300,true)", v, ok)
	}

	m.Set(1, 999) // overwrite
	if v, ok := m.Get(1); !ok || v != 999 {
		t.Fatalf("Get(1) after overwrite: got (%v,%v), want (999,true)", v, ok)
	}
	if m.Len() != 6 {
		t.Fatalf("Len after overwrite: got %d, want 6", m.Len())
	}

	if v, ok := m.Del(1); !ok || v != 999 {
		t.Fatalf("Del(1): got (%v,%v), want (999,true)", v, ok)
	}
	if m.Len() != 5 {
		t.Fatalf("Len after Del: got %d, want 5", m.Len())
	}
}

func TestOrderedMap_RangeOrdered(t *testing.T) {
	m := newOrderedMap()
	// Insert in shuffled order; Range must still yield ascending keys.
	for _, k := range []int{7, 2, 9, 0, 5, 3, 8, 1, 6, 4} {
		m.Set(k, k)
	}
	want := 2
	m.Range(2, 8, func(k, v int) {
		if k != want || v != want {
			t.Fatalf("Range: got (%d,%d), want key %d", k, v, want)
		}
		want++
	})
	if want != 8 {
		t.Fatalf("Range [2,8) ended at %d, want 8", want)
	}
}

// TestOrderedMap_Concurrent inserts and deletes over disjoint key ranges from
// many goroutines, then verifies the deterministic surviving set. Run with
// -race for memory safety.
func TestOrderedMap_Concurrent(t *testing.T) {
	const workers = 8
	const perWorker = 4000
	m := newOrderedMap()

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(base int) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(base)+1, 0x9e3779b9))
			for i := 0; i < perWorker; i++ {
				m.Set(base+i, base+i)
			}
			for i := 0; i < perWorker; i++ {
				if r.IntN(2) == 0 {
					m.Del(base + i)
				}
			}
		}(w * perWorker)
	}
	wg.Wait()

	// Re-derive the expected surviving set deterministically from the same seeds.
	expected := 0
	for w := 0; w < workers; w++ {
		base := w * perWorker
		r := rand.New(rand.NewPCG(uint64(base)+1, 0x9e3779b9))
		for i := 0; i < perWorker; i++ {
			deleted := r.IntN(2) == 0
			_, ok := m.Get(base + i)
			if deleted && ok {
				t.Fatalf("key %d present, expected deleted", base+i)
			}
			if !deleted {
				if !ok {
					t.Fatalf("key %d missing, expected present", base+i)
				}
				expected++
			}
		}
	}
	if got := m.Len(); got != expected {
		t.Fatalf("Len: got %d, want %d", got, expected)
	}
}
