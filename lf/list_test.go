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

func newList() *lf.List[int, int] {
	return lf.NewList[int, int](func(a, b int) bool { return a < b })
}

// Differential and concurrent-disjoint behavior are covered by the shared Map
// conformance suite (see conformance_test.go). The tests here cover list
// specifics: Contains, ordered Range, and same-key contention.

func TestList_ContainsAndRange(t *testing.T) {
	l := newList()
	if l.Contains(1) {
		t.Fatalf("empty list contains 1")
	}
	for _, k := range []int{5, 1, 3, 2, 4} { // inserted out of order
		l.Set(k, k*10)
	}
	if !l.Contains(3) {
		t.Fatalf("missing key 3")
	}
	if l.Len() != 5 {
		t.Fatalf("Len: got %d, want 5", l.Len())
	}
	want := 1
	l.Range(1, 5, func(k, v int) { // [1,5): keys 1..4 in order
		if k != want || v != want*10 {
			t.Fatalf("Range: got (%d,%d), want key %d", k, v, want)
		}
		want++
	})
	if want != 5 {
		t.Fatalf("Range ended at %d, want 5", want)
	}
}

// TestList_ContendedSameKey hammers a small shared keyspace from many goroutines
// so concurrent operations collide on the same key, the case where the
// marked-pointer unlink/help logic is exercised. It asserts invariants that hold
// regardless of which operation wins each race: Range stays strictly ascending
// (no duplicate or stale nodes) and Len agrees with the Range count.
func TestList_ContendedSameKey(t *testing.T) {
	const workers = 16
	const opsPer = 8000
	const keyspace = 48
	l := newList()

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
					l.Set(k, k*10)
				case 1:
					l.Del(k)
				default:
					if v, ok := l.Get(k); ok && v != k*10 {
						t.Errorf("Get(%d) returned corrupt value %d", k, v)
						return
					}
				}
			}
		}(uint64(w) + 1)
	}
	wg.Wait()

	var count, prev int
	first := true
	l.Range(0, keyspace, func(k, v int) {
		if !first && k <= prev {
			t.Fatalf("Range not strictly ascending: %d after %d", k, prev)
		}
		if v != k*10 {
			t.Fatalf("Range key %d has corrupt value %d", k, v)
		}
		first = false
		prev = k
		count++
	})
	if got := l.Len(); got != count {
		t.Fatalf("Len %d disagrees with Range count %d", got, count)
	}
}
