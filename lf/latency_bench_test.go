// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"math/bits"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.design/x/lockfree/lf"
	"golang.design/x/lockfree/wf"
)

// latHist is a coarse power-of-two latency histogram (bucket i counts samples
// whose nanosecond cost has bit length i, i.e. in [2^(i-1), 2^i)). It records
// without allocation or locking so per-operation timing stays cheap; the exact
// worst case is kept separately because that is the headline for a latency bound.
type latHist struct {
	buckets [64]uint64
	max     int64
}

func (h *latHist) add(ns int64) {
	if ns > h.max {
		h.max = ns
	}
	h.buckets[bits.Len64(uint64(ns))]++
}

func (h *latHist) merge(o *latHist) {
	if o.max > h.max {
		h.max = o.max
	}
	for i := range h.buckets {
		h.buckets[i] += o.buckets[i]
	}
}

// percentile returns the upper bound (2^bucket ns) of the bucket holding the
// p-th percentile. Granularity is a factor of two, enough to compare tails that
// differ by an order of magnitude.
func (h *latHist) percentile(p float64) int64 {
	var total uint64
	for _, c := range h.buckets {
		total += c
	}
	target := uint64(p * float64(total))
	var cum uint64
	for i, c := range h.buckets {
		cum += c
		if cum >= target {
			return int64(1) << uint(i)
		}
	}
	return h.max
}

// measureLatency times every operation across an oversubscribed goroutine pool
// (oversub times GOMAXPROCS goroutines, so most are preempted mid-flight) and
// reports the latency distribution as custom metrics. bind is called once per
// goroutine and returns that goroutine's single-operation closure.
func measureLatency(b *testing.B, oversub int, bind func() func()) {
	var mu sync.Mutex
	var hists []*latHist
	b.SetParallelism(oversub)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		op := bind()
		h := &latHist{}
		mu.Lock()
		hists = append(hists, h)
		mu.Unlock()
		for pb.Next() {
			start := time.Now()
			op()
			h.add(int64(time.Since(start)))
		}
	})
	b.StopTimer()
	agg := &latHist{}
	for _, h := range hists {
		agg.merge(h)
	}
	b.ReportMetric(float64(agg.percentile(0.50)), "p50-ns")
	b.ReportMetric(float64(agg.percentile(0.99)), "p99-ns")
	b.ReportMetric(float64(agg.percentile(0.999)), "p999-ns")
	b.ReportMetric(float64(agg.max), "max-ns")
}

// BenchmarkQueueLatency measures the wall-clock per-operation latency tail of the
// lock-free and wait-free queues under heavy oversubscription.
//
// Read this as a documented negative result, not as evidence about the wait-free
// guarantee. One might expect the wait-free queue to win the tail, since its
// guarantee is that every operation finishes in a bounded number of steps and so
// no thread starves. It does not: in practice its p999 and max are no better
// (often worse) than the lock-free queue's.
//
// The reason is that this benchmark cannot measure what the guarantee is about.
// The bound is on a thread's own step count, not wall-clock time. Under the
// oversubscription that the guarantee is designed to survive, most goroutines are
// descheduled at any instant, so a sample taken around a single operation
// includes however long the goroutine sat on the run queue, plus any GC pause.
// That descheduling and GC time dominates the tail and is unrelated to the
// operation; it is in fact heavier for the allocation-heavier wait-free code.
// Removing the oversubscription would remove the adversarial scheduling the
// guarantee addresses, so there is no oversubscription setting that turns this
// into an honest measurement of the property. The wait-free latency bound lives
// in the analysis, the same way the progress guarantees do. This benchmark is
// kept so the negative result is reproducible: run it and see the tail is about
// the scheduler, not the algorithm.
func BenchmarkQueueLatency(b *testing.B) {
	const prefill = 1024
	const oversub = 8
	numG := oversub * runtime.GOMAXPROCS(0)
	impls := []struct {
		name  string
		setup func(maxParticipants int) func() func()
	}{
		{"lockfree", func(int) func() func() {
			q := lf.NewQueue[int]()
			for i := 0; i < prefill; i++ {
				q.Enqueue(i)
			}
			return func() func() {
				on := false
				return func() {
					on = !on
					if on {
						q.Enqueue(1)
					} else {
						q.Dequeue()
					}
				}
			}
		}},
		{"waitfree", func(maxParticipants int) func() func() {
			q := wf.NewQueue[int](maxParticipants)
			pre := q.Handle()
			for i := 0; i < prefill; i++ {
				pre.Enqueue(i)
			}
			return func() func() {
				h := q.Handle()
				on := false
				return func() {
					on = !on
					if on {
						h.Enqueue(1)
					} else {
						h.Dequeue()
					}
				}
			}
		}},
	}
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			measureLatency(b, oversub, impl.setup(numG+1))
		})
	}
}
