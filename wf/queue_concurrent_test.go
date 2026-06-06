// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree/wf"
)

// pair tags a value with the producer that enqueued it and that producer's
// per-item sequence number, so tests can check FIFO ordering, not just totals.
type pair struct {
	p, s int
}

// TestQueueConcurrentFIFO stresses the queue with many concurrent producers and
// a SINGLE consumer, so the consumer observes the exact global dequeue order.
// Each producer enqueues 0,1,2,... in order, so within the dequeue stream each
// producer's sequence numbers must appear strictly increasing by one — a real
// linearizability check that bare conservation would miss.
//
// Handles are oversubscribed relative to GOMAXPROCS so that goroutines are
// preempted mid-operation and are forced to complete a stalled peer's operation
// end-to-end, exercising the wait-free helping mechanism. Run with -race;
// repeat with -count for timing coverage.
func TestQueueConcurrentFIFO(t *testing.T) {
	const producers = 24
	const perProducer = 4000
	const expected = producers * perProducer

	q := wf.NewQueue[pair](producers + 1)           // +1 for the consumer handle
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4)) // handles (25) >> procs (4); restore after

	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		go func(p int) {
			defer wg.Done()
			h := q.Handle()
			for s := 0; s < perProducer; s++ {
				h.Enqueue(pair{p: p, s: s})
			}
		}(p)
	}

	h := q.Handle()
	last := make([]int, producers)
	for i := range last {
		last[i] = -1
	}
	got := 0
	for got < expected {
		v, ok := h.Dequeue()
		if !ok {
			runtime.Gosched()
			continue
		}
		if v.s != last[v.p]+1 {
			t.Fatalf("FIFO violated for producer %d: got seq %d after %d", v.p, v.s, last[v.p])
		}
		last[v.p] = v.s
		got++
	}
	wg.Wait()
	if v, ok := h.Dequeue(); ok {
		t.Fatalf("queue not drained: leftover %v", v)
	}
}

// TestQueueConcurrentConservation runs the full multi-producer/multi-consumer
// case and asserts every enqueued value is dequeued exactly once — none lost,
// none duplicated. Handles are oversubscribed to force helping. Run with -race.
func TestQueueConcurrentConservation(t *testing.T) {
	const producers = 24
	const consumers = 8
	const perProducer = 4000
	const expected = producers * perProducer

	q := wf.NewQueue[int](producers + consumers)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))

	var pg sync.WaitGroup
	pg.Add(producers)
	for p := 0; p < producers; p++ {
		go func(p int) {
			defer pg.Done()
			h := q.Handle()
			base := p * perProducer
			for s := 0; s < perProducer; s++ {
				h.Enqueue(base + s) // globally unique value
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
			h := q.Handle()
			var mine []int
			for {
				v, ok := h.Dequeue()
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
