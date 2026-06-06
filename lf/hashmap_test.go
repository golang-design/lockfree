// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"sync"
	"testing"

	"golang.design/x/lockfree/lf"
)

func newHashMap(buckets int) *lf.HashMap[int, int] {
	return lf.NewHashMap[int, int](buckets, func(k int) uint64 { return uint64(k) * 0x9e3779b97f4a7c15 },
		func(a, b int) bool { return a < b })
}

// Behavior is covered by the shared Map conformance suite (conformance_test.go).
// These cover hash-table specifics: construction validation and heavy collision.

func TestNewHashMapInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for numBuckets < 1")
		}
	}()
	newHashMap(0)
}

func TestHashMap_BasicContains(t *testing.T) {
	m := newHashMap(8)
	if m.Contains(1) {
		t.Fatalf("empty map contains 1")
	}
	m.Set(1, 10)
	if v, ok := m.Get(1); !ok || v != 10 {
		t.Fatalf("Get(1): got (%v,%v), want (10,true)", v, ok)
	}
	if !m.Contains(1) {
		t.Fatalf("missing key 1")
	}
	if v, ok := m.Del(1); !ok || v != 10 {
		t.Fatalf("Del(1): got (%v,%v), want (10,true)", v, ok)
	}
	if m.Contains(1) {
		t.Fatalf("key 1 present after Del")
	}
}

// TestHashMap_HighCollision uses very few buckets so many goroutines contend on
// the same bucket lists, stressing the lock-free bucket operations. Keys are
// disjoint per worker, so the surviving set is deterministic.
func TestHashMap_HighCollision(t *testing.T) {
	const buckets = 4
	const workers = 8
	const perWorker = 2000
	m := newHashMap(buckets)

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
