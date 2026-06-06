// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package conformtest

import (
	"math/rand/v2"
	"sync"
	"testing"

	"golang.design/x/lockfree"
)

// MapFactory builds a map supporting up to maxParticipants concurrent
// participants and returns a function that yields one participant's view. A
// lock-free map (unbounded participants) returns the same value every time; a
// wait-free map returns a fresh per-goroutine handle from the budget.
type MapFactory func(maxParticipants int) (participant func() lockfree.Map[int, int])

// Map runs the full behavioral conformance suite for a key/value map against the
// given implementation.
func Map(t *testing.T, factory MapFactory) {
	t.Helper()
	t.Run("Differential", func(t *testing.T) { mapDifferential(t, factory) })
	t.Run("ConcurrentDisjoint", func(t *testing.T) { mapConcurrentDisjoint(t, factory) })
}

// mapDifferential replays a long random op sequence against a builtin map.
func mapDifferential(t *testing.T, factory MapFactory) {
	// Sizes are kept modest so the suite is also reasonable for O(n)
	// implementations such as the list-based map; implementation-specific heavy
	// stress lives in each type's own tests.
	const ops = 50000
	const keyspace = 256
	m := factory(1)()
	ref := map[int]int{}
	rng := rand.New(rand.NewPCG(1, 2))

	for i := 0; i < ops; i++ {
		k := int(rng.IntN(keyspace))
		switch rng.IntN(3) {
		case 0: // Set
			v := int(rng.Int())
			m.Set(k, v)
			ref[k] = v
		case 1: // Del
			wv, wok := ref[k]
			delete(ref, k)
			gv, gok := m.Del(k)
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("Del(%d): got (%v,%v) want (%v,%v)", k, gv, gok, wv, wok)
			}
		case 2: // Get
			wv, wok := ref[k]
			gv, gok := m.Get(k)
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("Get(%d): got (%v,%v) want (%v,%v)", k, gv, gok, wv, wok)
			}
		}
	}
	for k := 0; k < keyspace; k++ { // full sweep: every key must agree
		wv, wok := ref[k]
		gv, gok := m.Get(k)
		if gok != wok || (wok && gv != wv) {
			t.Fatalf("final Get(%d): got (%v,%v) want (%v,%v)", k, gv, gok, wv, wok)
		}
	}
}

// mapConcurrentDisjoint has each worker own a disjoint key range so the final
// state is deterministic despite concurrency: each worker sets its keys then
// deletes the even offsets, and the odd offsets must survive.
func mapConcurrentDisjoint(t *testing.T, factory MapFactory) {
	const workers = 8
	const perWorker = 300
	participant := factory(workers + 1)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(base int) {
			defer wg.Done()
			m := participant()
			for i := 0; i < perWorker; i++ {
				m.Set(base+i, base+i)
			}
			for i := 0; i < perWorker; i += 2 {
				m.Del(base + i)
			}
		}(w * perWorker)
	}
	wg.Wait()

	m := participant()
	for w := 0; w < workers; w++ {
		base := w * perWorker
		for i := 0; i < perWorker; i++ {
			v, ok := m.Get(base + i)
			if i%2 == 1 { // odd offsets survive
				if !ok || v != base+i {
					t.Fatalf("key %d: got (%v,%v), want present", base+i, v, ok)
				}
			} else if ok { // even offsets deleted
				t.Fatalf("key %d present, expected deleted", base+i)
			}
		}
	}
}
