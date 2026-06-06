// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree/lf"
)

func newSkipList() *lf.SkipList[int, int] {
	return lf.NewSkipList[int, int](func(a, b int) bool { return a < b })
}

func TestSkipList_Len(t *testing.T) {
	sl := newSkipList()
	if got := sl.Len(); got != 0 {
		t.Fatalf("Len: got %d, want %d", got, 0)
	}
	for i := 0; i < 10000; i++ {
		sl.Set(i, i)
	}
	if got := sl.Len(); got != 10000 {
		t.Fatalf("Len: got %d, want %d", got, 10000)
	}
}

func TestSkipList_GetFail(t *testing.T) {
	sl := newSkipList()
	if v, ok := sl.Get(-1); ok {
		t.Fatalf("expected miss, got %v, %v", v, ok)
	}
}

func TestSkipList_GetSuccess(t *testing.T) {
	sl := newSkipList()
	sl.Set(1, 2)
	if got, ok := sl.Get(1); got != 2 || !ok {
		t.Fatalf("got %v, %v want %v, %v", got, ok, 2, true)
	}
	sl.Set(1, 3) // update existing key
	if got, ok := sl.Get(1); got != 3 || !ok {
		t.Fatalf("got %v, %v want %v, %v", got, ok, 3, true)
	}
	if got := sl.Len(); got != 1 {
		t.Fatalf("Len after update: got %d, want 1", got)
	}
}

func TestSkipList_Search(t *testing.T) {
	sl := newSkipList()
	if sl.Search(1) {
		t.Fatalf("Search on empty list returned true")
	}
	sl.Set(1, 2)
	if !sl.Search(1) {
		t.Fatalf("Search after Set returned false")
	}
	if v, ok := sl.Del(1); v != 2 || !ok {
		t.Fatalf("Del got %v,%v want 2,true", v, ok)
	}
	if sl.Search(1) {
		t.Fatalf("Search after Del returned true")
	}
	if got := sl.Len(); got != 0 {
		t.Fatalf("Len: got %d, want 0", got)
	}
}

func TestSkipList_Del(t *testing.T) {
	sl := newSkipList()
	for i := 0; i < 10; i++ {
		sl.Set(i, i)
	}
	for i := 0; i < 100; i++ {
		v, ok := sl.Del(i)
		if i < 10 && (!ok || v != i) {
			t.Fatalf("Del(%d): got %v,%v want %d,true", i, v, ok, i)
		}
		if i >= 10 && ok {
			t.Fatalf("Del(%d): expected miss, got %v", i, ok)
		}
	}
	if got := sl.Len(); got != 0 {
		t.Fatalf("Len: got %d, want 0", got)
	}
}

func TestSkipList_Range(t *testing.T) {
	sl := newSkipList()
	for i := 0; i < 100; i++ {
		sl.Set(i, i)
	}
	current := 10
	sl.Range(10, 20, func(k, v int) {
		if v != current {
			t.Fatalf("range failed, want %v, got %v", current, v)
		}
		current++
	})
	if current != 20 {
		t.Fatalf("range [10,20) ended at %d, want 20", current)
	}

	current = 90
	sl.Range(90, 120, func(k, v int) {
		if v != current {
			t.Fatalf("range failed, want %v, got %v", current, v)
		}
		current++
	})
	if current != 100 {
		t.Fatalf("range [90,120) ended at %d, want 100", current)
	}
}

// TestSkipList_Differential fuzzes the lock-free skip list against a plain map
// reference over a long random sequence of operations and asserts identical
// observable behavior. Run sequentially so the comparison is deterministic.
func TestSkipList_Differential(t *testing.T) {
	const ops = 200000
	const keyspace = 256
	rng := rand.New(rand.NewPCG(1, 2))
	sl := newSkipList()
	ref := map[int]int{}

	for i := 0; i < ops; i++ {
		k := int(rng.IntN(keyspace))
		switch rng.IntN(3) {
		case 0: // Set
			v := int(rng.Int())
			sl.Set(k, v)
			ref[k] = v
		case 1: // Del
			wv, wok := ref[k]
			delete(ref, k)
			gv, gok := sl.Del(k)
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("Del(%d): got (%v,%v) want (%v,%v)", k, gv, gok, wv, wok)
			}
		case 2: // Get
			wv, wok := ref[k]
			gv, gok := sl.Get(k)
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("Get(%d): got (%v,%v) want (%v,%v)", k, gv, gok, wv, wok)
			}
		}
	}
	if got := sl.Len(); got != len(ref) {
		t.Fatalf("Len: got %d, want %d", got, len(ref))
	}
	// Full sweep: every key must agree.
	for k := 0; k < keyspace; k++ {
		wv, wok := ref[k]
		gv, gok := sl.Get(k)
		if gok != wok || (wok && gv != wv) {
			t.Fatalf("final Get(%d): got (%v,%v) want (%v,%v)", k, gv, gok, wv, wok)
		}
	}
}

// TestSkipList_Concurrent runs many goroutines over disjoint key ranges so the
// final state is deterministic despite concurrency: each goroutine inserts its
// keys, deletes the even ones, and the odd ones must survive. Run with -race to
// check memory safety. (-race proves safety, not lock-freedom; see SkipList's
// doc comment for that argument.)
func TestSkipList_Concurrent(t *testing.T) {
	const workers = 8
	const perWorker = 5000
	sl := newSkipList()

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(base int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				sl.Set(base+i, base+i)
			}
			for i := 0; i < perWorker; i += 2 { // delete even offsets
				sl.Del(base + i)
			}
		}(w * perWorker)
	}
	wg.Wait()

	var survivors int
	for w := 0; w < workers; w++ {
		base := w * perWorker
		for i := 0; i < perWorker; i++ {
			_, ok := sl.Get(base + i)
			if i%2 == 1 { // odd offsets survive
				if !ok {
					t.Fatalf("key %d missing, expected present", base+i)
				}
				survivors++
			} else if ok {
				t.Fatalf("key %d present, expected deleted", base+i)
			}
		}
	}
	if got := sl.Len(); got != survivors {
		t.Fatalf("Len: got %d, want %d survivors", got, survivors)
	}
}

// TestSkipList_ContendedSameKey hammers a small shared keyspace from many
// goroutines with random Set/Del/Get so that concurrent operations collide on
// the SAME key — the paths that disjoint-key tests never reach (two Sets racing
// one absent key, two Dels racing one present key, Set/Del helping in find).
// The winner of any race is nondeterministic, so it asserts only invariants
// that must hold regardless: keys stay strictly ascending (no duplicate nodes,
// no ordering corruption) and the Range count equals Len (no counter drift or
// failed unlink). Runs under -race; use -count to repeat for timing coverage.
func TestSkipList_ContendedSameKey(t *testing.T) {
	const workers = 16
	const opsPer = 20000
	const keyspace = 64
	sl := newSkipList()

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
					sl.Set(k, k*10)
				case 1:
					sl.Del(k)
				default:
					if v, ok := sl.Get(k); ok && v != k*10 {
						t.Errorf("Get(%d) returned corrupt value %d", k, v)
						return
					}
				}
			}
		}(uint64(w) + 1)
	}
	wg.Wait()

	// Invariants that hold regardless of who won each race.
	var (
		count int
		prev  int
		first = true
	)
	sl.Range(0, keyspace, func(k, v int) {
		if !first && k <= prev {
			t.Fatalf("Range not strictly ascending: %d after %d", k, prev)
		}
		if v != k*10 {
			t.Fatalf("Range key %d has corrupt value %d", k, v)
		}
		if k < 0 || k >= keyspace {
			t.Fatalf("Range key %d out of keyspace", k)
		}
		first = false
		prev = k
		count++
	})
	if got := sl.Len(); got != count {
		t.Fatalf("Len %d disagrees with Range count %d (counter drift or stale node)", got, count)
	}
}

// skipListInterface lets the benchmark drive both implementations identically.
type skipListInterface interface {
	Set(int, int)
	Get(int) (int, bool)
	Del(int) (int, bool)
}

// mutexMap is the mutex-guarded baseline for the benchmark.
type mutexMap struct {
	mu sync.Mutex
	m  map[int]int
}

func newMutexMap() *mutexMap { return &mutexMap{m: map[int]int{}} }

func (s *mutexMap) Set(k, v int) {
	s.mu.Lock()
	s.m[k] = v
	s.mu.Unlock()
}

func (s *mutexMap) Get(k int) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	return v, ok
}

func (s *mutexMap) Del(k int) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	delete(s.m, k)
	return v, ok
}

// BenchmarkSkipList compares the lock-free skip list against a mutex-guarded map
// under a read-heavy mixed workload.
func BenchmarkSkipList(b *testing.B) {
	const keyspace = 1 << 16
	impls := []struct {
		name string
		s    skipListInterface
	}{
		{"lockfree", newSkipList()},
		{"mutex", newMutexMap()},
	}
	for _, impl := range impls {
		for i := 0; i < keyspace; i++ { // warm up
			impl.s.Set(i, i)
		}
		b.Run(impl.name, func(b *testing.B) {
			var c int64
			b.RunParallel(func(pb *testing.PB) {
				r := rand.Uint64()
				for pb.Next() {
					r ^= r << 13
					r ^= r >> 7
					r ^= r << 17 // xorshift for a cheap per-iteration key
					k := int(r % keyspace)
					switch atomic.AddInt64(&c, 1) & 7 {
					case 0:
						impl.s.Set(k, k)
					case 1:
						impl.s.Del(k)
					default:
						impl.s.Get(k)
					}
				}
			})
		})
	}
}
