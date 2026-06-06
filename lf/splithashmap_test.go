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

func newSplitHashMap() *lf.SplitHashMap[int, int] {
	return lf.NewSplitHashMap[int, int](func(k int) uint64 { return uint64(k) * 0x9e3779b97f4a7c15 },
		func(a, b int) bool { return a < b })
}

// Behavior is covered by the shared Map conformance suite and resizing by the
// white-box TestSplitHashMapGrows. These cover basics and concurrent growth.

func TestSplitHashMap_BasicContains(t *testing.T) {
	m := newSplitHashMap()
	if m.Contains(1) {
		t.Fatalf("empty map contains 1")
	}
	m.Set(1, 10)
	m.Set(1, 20) // update
	if v, ok := m.Get(1); !ok || v != 20 {
		t.Fatalf("Get(1): got (%v,%v), want (20,true)", v, ok)
	}
	if m.Len() != 1 {
		t.Fatalf("Len after update: got %d, want 1", m.Len())
	}
	if v, ok := m.Del(1); !ok || v != 20 {
		t.Fatalf("Del(1): got (%v,%v), want (20,true)", v, ok)
	}
	if m.Contains(1) {
		t.Fatalf("key present after Del")
	}
}

// TestSplitHashMap_ConcurrentGrowth inserts disjoint key ranges from many
// goroutines, forcing concurrent table growth, then verifies the surviving set.
func TestSplitHashMap_ConcurrentGrowth(t *testing.T) {
	const workers = 8
	const perWorker = 3000
	m := newSplitHashMap()

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(base int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				m.Set(base+i, base+i)
			}
			for i := 0; i < perWorker; i += 2 {
				m.Del(base + i)
			}
		}(w * perWorker)
	}
	wg.Wait()

	survivors := 0
	for w := 0; w < workers; w++ {
		base := w * perWorker
		for i := 0; i < perWorker; i++ {
			v, ok := m.Get(base + i)
			if i%2 == 1 {
				if !ok || v != base+i {
					t.Fatalf("key %d: got (%v,%v), want present", base+i, v, ok)
				}
				survivors++
			} else if ok {
				t.Fatalf("key %d present, expected deleted", base+i)
			}
		}
	}
	if got := m.Len(); got != survivors {
		t.Fatalf("Len: got %d, want %d", got, survivors)
	}
}

// TestSplitHashMap_ContendedSameKey hammers a small shared keyspace so many
// goroutines collide on the same keys. SplitHashMap implements Del and the count
// independently (it does not delegate to List), so this is the test class that
// caught the List double-decrement bug: after the storm Len must equal the number
// of keys actually present.
func TestSplitHashMap_ContendedSameKey(t *testing.T) {
	const workers = 16
	const opsPer = 8000
	const keyspace = 64
	m := newSplitHashMap()

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(seed uint64) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(seed, seed*2654435761+1))
			for i := 0; i < opsPer; i++ {
				k := int(r.IntN(keyspace))
				switch r.IntN(3) {
				case 0:
					m.Set(k, k*10)
				case 1:
					m.Del(k)
				default:
					if v, ok := m.Get(k); ok && v != k*10 {
						t.Errorf("Get(%d) corrupt value %d", k, v)
						return
					}
				}
			}
		}(uint64(w) + 1)
	}
	wg.Wait()

	present := 0
	for k := 0; k < keyspace; k++ {
		if _, ok := m.Get(k); ok {
			present++
		}
	}
	if got := m.Len(); got != present {
		t.Fatalf("Len %d disagrees with %d keys present (counter drift)", got, present)
	}
}

// TestSplitHashMap_Collisions uses a hash with only 4 distinct values so many
// keys share a split-order key, exercising the less-based tiebreak in nodeLess /
// nodeEqual that the well-spread conformance hashes never reach.
func TestSplitHashMap_Collisions(t *testing.T) {
	m := lf.NewSplitHashMap[int, int](func(k int) uint64 { return uint64(k % 4) },
		func(a, b int) bool { return a < b })
	const n = 2000
	for i := 0; i < n; i++ {
		m.Set(i, i*2)
	}
	if m.Len() != n {
		t.Fatalf("Len: got %d, want %d", m.Len(), n)
	}
	for i := 0; i < n; i++ {
		if v, ok := m.Get(i); !ok || v != i*2 {
			t.Fatalf("Get(%d): got (%v,%v), want (%d,true)", i, v, ok, i*2)
		}
	}
	for i := 0; i < n; i += 2 {
		if _, ok := m.Del(i); !ok {
			t.Fatalf("Del(%d) missing", i)
		}
	}
	for i := 0; i < n; i++ {
		_, ok := m.Get(i)
		if want := i%2 == 1; ok != want {
			t.Fatalf("Get(%d): present=%v, want %v", i, ok, want)
		}
	}
}
