// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

// Package conformtest holds behavioral conformance suites that every
// implementation of a given abstract data type must pass. A single suite is run
// against each implementation (e.g. both the lock-free and the wait-free queue)
// so that all variants are verified to behave identically.
package conformtest

import (
	"math/rand/v2"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree"
)

// QueueFactory builds a queue supporting up to maxParticipants concurrent
// participants and returns a function that yields one participant's view of the
// queue. The yielded function must be called once per goroutine: a lock-free
// queue (unbounded participants) returns the same value every time, while a
// wait-free queue returns a fresh per-goroutine handle.
type QueueFactory func(maxParticipants int) (participant func() lockfree.Queue[int])

// Queue runs the full behavioral conformance suite for a FIFO queue against the
// given implementation.
func Queue(t *testing.T, factory QueueFactory) {
	t.Helper()
	t.Run("Sequential", func(t *testing.T) { queueSequential(t, factory) })
	t.Run("Differential", func(t *testing.T) { queueDifferential(t, factory) })
	t.Run("ConcurrentFIFO", func(t *testing.T) { queueConcurrentFIFO(t, factory) })
	t.Run("ConcurrentConservation", func(t *testing.T) { queueConcurrentConservation(t, factory) })
}

// queueSequential checks FIFO ordering and empty behavior with one participant.
func queueSequential(t *testing.T, factory QueueFactory) {
	q := factory(1)()
	if v, ok := q.Dequeue(); ok {
		t.Fatalf("dequeue empty returned (%v, true)", v)
	}
	for i := 0; i < 100; i++ {
		q.Enqueue(i)
	}
	for i := 0; i < 100; i++ {
		v, ok := q.Dequeue()
		if !ok || v != i {
			t.Fatalf("dequeue: got (%v,%v), want (%d,true)", v, ok, i)
		}
	}
	if v, ok := q.Dequeue(); ok {
		t.Fatalf("dequeue drained returned (%v, true)", v)
	}
}

// queueDifferential replays a long random op sequence against a slice reference.
func queueDifferential(t *testing.T, factory QueueFactory) {
	const ops = 100000
	q := factory(1)()
	rng := rand.New(rand.NewPCG(1, 2))
	var ref []int
	for i := 0; i < ops; i++ {
		if rng.IntN(2) == 0 {
			v := int(rng.Int())
			q.Enqueue(v)
			ref = append(ref, v)
		} else {
			gv, gok := q.Dequeue()
			if len(ref) == 0 {
				if gok {
					t.Fatalf("dequeue empty: got (%v, true)", gv)
				}
				continue
			}
			wv := ref[0]
			ref = ref[1:]
			if !gok || gv != wv {
				t.Fatalf("dequeue: got (%v,%v), want (%v,true)", gv, gok, wv)
			}
		}
	}
}

// queueConcurrentFIFO uses many producers and a single consumer (so the consumer
// observes the exact global dequeue order) and asserts per-producer FIFO. Each
// value encodes producer*perProducer + seq. Participants are oversubscribed vs
// GOMAXPROCS to force mid-operation preemption (and, for wait-free, helping).
func queueConcurrentFIFO(t *testing.T, factory QueueFactory) {
	const producers = 16
	const perProducer = 4000
	const expected = producers * perProducer
	participant := factory(producers + 1)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))

	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		go func(p int) {
			defer wg.Done()
			q := participant()
			for s := 0; s < perProducer; s++ {
				q.Enqueue(p*perProducer + s)
			}
		}(p)
	}

	q := participant()
	last := make([]int, producers)
	for i := range last {
		last[i] = -1
	}
	for got := 0; got < expected; {
		v, ok := q.Dequeue()
		if !ok {
			runtime.Gosched()
			continue
		}
		p, s := v/perProducer, v%perProducer
		if s != last[p]+1 {
			t.Fatalf("FIFO violated for producer %d: got seq %d after %d", p, s, last[p])
		}
		last[p] = s
		got++
	}
	wg.Wait()
	if v, ok := q.Dequeue(); ok {
		t.Fatalf("queue not drained: leftover %v", v)
	}
}

// queueConcurrentConservation runs full MPMC and asserts every value is dequeued
// exactly once.
func queueConcurrentConservation(t *testing.T, factory QueueFactory) {
	const producers = 16
	const consumers = 8
	const perProducer = 4000
	const expected = producers * perProducer
	participant := factory(producers + consumers)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))

	var pg sync.WaitGroup
	pg.Add(producers)
	for p := 0; p < producers; p++ {
		go func(p int) {
			defer pg.Done()
			q := participant()
			base := p * perProducer
			for s := 0; s < perProducer; s++ {
				q.Enqueue(base + s)
			}
		}(p)
	}

	var consumed atomic.Int64
	results := make([][]int, consumers)
	var cg sync.WaitGroup
	cg.Add(consumers)
	for c := 0; c < consumers; c++ {
		go func(c int) {
			defer cg.Done()
			q := participant()
			var mine []int
			for {
				v, ok := q.Dequeue()
				if ok {
					mine = append(mine, v)
					consumed.Add(1)
					continue
				}
				if consumed.Load() >= expected {
					results[c] = mine
					return
				}
				runtime.Gosched()
			}
		}(c)
	}
	pg.Wait()
	cg.Wait()

	seen := make([]int32, expected)
	total := 0
	for _, mine := range results {
		for _, v := range mine {
			if v < 0 || v >= expected {
				t.Fatalf("dequeued out-of-range value %d", v)
			}
			seen[v]++
			total++
		}
	}
	if total != expected {
		t.Fatalf("conservation: dequeued %d, want %d", total, expected)
	}
	for v, n := range seen {
		if n != 1 {
			t.Fatalf("value %d dequeued %d times, want exactly 1", v, n)
		}
	}
}
