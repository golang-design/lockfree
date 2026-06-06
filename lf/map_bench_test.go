// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"math/rand/v2"
	"sync"
	"testing"

	"golang.design/x/lockfree"
	"golang.design/x/lockfree/lf"
)

// mutexMap is a plain sync.Mutex around a builtin map.
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

// rwMutexMap takes a read lock for Get so concurrent readers do not serialize.
type rwMutexMap struct {
	mu sync.RWMutex
	m  map[int]int
}

func newRWMutexMap() *rwMutexMap { return &rwMutexMap{m: map[int]int{}} }

func (s *rwMutexMap) Set(k, v int) {
	s.mu.Lock()
	s.m[k] = v
	s.mu.Unlock()
}

func (s *rwMutexMap) Get(k int) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[k]
	return v, ok
}

func (s *rwMutexMap) Del(k int) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	delete(s.m, k)
	return v, ok
}

// syncMapAdapter wraps the stdlib sync.Map. Del uses LoadAndDelete so it returns
// the old value, matching the work the other implementations do on a delete (a
// bare Delete would make sync.Map look artificially cheap on writes).
type syncMapAdapter struct{ m sync.Map }

func (s *syncMapAdapter) Set(k, v int) { s.m.Store(k, v) }

func (s *syncMapAdapter) Get(k int) (int, bool) {
	v, ok := s.m.Load(k)
	if !ok {
		return 0, false
	}
	return v.(int), true
}

func (s *syncMapAdapter) Del(k int) (int, bool) {
	v, ok := s.m.LoadAndDelete(k)
	if !ok {
		return 0, false
	}
	return v.(int), true
}

// mapBody returns a RunParallel body running a mixed map workload. Each
// goroutine seeds its own xorshift stream (distinct sequences avoid artificial
// same-key contention) and draws the key from the high bits and the operation
// from the low bits, so key and operation are uncorrelated. Read-heavy is ~7/8
// Get with 1/16 Set and 1/16 Del; write-heavy is 1/2 Get with 1/4 Set and 1/4
// Del, which drains the prefilled map toward half full.
func mapBody(m lockfree.Map[int, int], keyspace uint64, readHeavy bool) func(pb *testing.PB) {
	mask := keyspace - 1 // keyspace is a power of two
	return func(pb *testing.PB) {
		r := rand.Uint64() | 1 // nonzero seed for xorshift, distinct per goroutine
		for pb.Next() {
			r ^= r << 13
			r ^= r >> 7
			r ^= r << 17
			k := int((r >> 40) & mask)
			if readHeavy {
				switch r & 15 {
				case 0:
					m.Set(k, k)
				case 1:
					m.Del(k)
				default:
					m.Get(k)
				}
			} else {
				switch r & 3 {
				case 0:
					m.Set(k, k)
				case 1:
					m.Del(k)
				default:
					m.Get(k)
				}
			}
		}
	}
}

// BenchmarkMap compares the lock-free maps against three stdlib-style baselines
// (sync.Mutex map, sync.RWMutex map, sync.Map) under read-heavy and write-heavy
// mixes swept across goroutine counts.
//
// The Harris ordered List is deliberately excluded: it is an O(n) ordered linked
// list, a textbook building block rather than a scalable map, so comparing its
// throughput against O(1)/O(log n) maps would not be meaningful. OrderedMap is
// also omitted because it is a thin facade over SkipList with identical numbers.
// HashMap is pre-sized to the keyspace, which is its best case; an undersized
// fixed-bucket map would instead degrade to per-bucket list scans.
func BenchmarkMap(b *testing.B) {
	const keyspace = 1 << 14
	less := func(a, b int) bool { return a < b }
	impls := []struct {
		name string
		make func() lockfree.Map[int, int]
	}{
		{"mutex", func() lockfree.Map[int, int] { return newMutexMap() }},
		{"rwmutex", func() lockfree.Map[int, int] { return newRWMutexMap() }},
		{"syncmap", func() lockfree.Map[int, int] { return &syncMapAdapter{} }},
		{"skiplist", func() lockfree.Map[int, int] { return newSkipList() }},
		{"hashmap", func() lockfree.Map[int, int] { return lf.NewHashMap[int, int](keyspace, hashInt, less) }},
		{"splithashmap", func() lockfree.Map[int, int] { return lf.NewSplitHashMap[int, int](hashInt, less) }},
	}
	mixes := []struct {
		name      string
		readHeavy bool
	}{
		{"readheavy", true},
		{"writeheavy", false},
	}
	for _, mix := range mixes {
		b.Run(mix.name, func(b *testing.B) {
			for _, impl := range impls {
				b.Run(impl.name, func(b *testing.B) {
					runSweep(b, func(int) func(*testing.PB) {
						m := impl.make()
						for i := 0; i < keyspace; i++ {
							m.Set(i, i)
						}
						return mapBody(m, keyspace, mix.readHeavy)
					})
				})
			}
		})
	}
}
