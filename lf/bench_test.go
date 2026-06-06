// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"fmt"
	"runtime"
	"testing"
)

// benchLevels is the sweep of goroutine counts used by the contention
// benchmarks. At or below the core count this measures scaling with real
// parallelism; above it the goroutines oversubscribe the cores and the numbers
// reflect scheduling contention.
var benchLevels = []int{1, 2, 4, 8, 16, 32}

// runSweep runs build once per goroutine-count level. build receives the maximum
// number of participants (goroutines plus one for prefill, for handle-based
// wait-free types) and returns the RunParallel body. Setting GOMAXPROCS to the
// level and parallelism to 1 makes RunParallel spawn exactly that many
// goroutines, so each level is an exact concurrency point.
func runSweep(b *testing.B, build func(maxParticipants int) func(pb *testing.PB)) {
	for _, g := range benchLevels {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(g))
			body := build(g + 1)
			b.SetParallelism(1)
			b.ResetTimer()
			b.RunParallel(body)
		})
	}
}

// balancedPushPop returns a RunParallel body that alternates a push and a pop
// using a per-goroutine local toggle. The toggle is deliberately local: a shared
// atomic op-selector would ping-pong one cache line across all goroutines and
// become the bottleneck at high goroutine counts, measuring the counter instead
// of the data structure. bind is called once per goroutine and returns that
// goroutine's push and pop closures (a fresh handle for wait-free types).
func balancedPushPop(bind func() (push func(), pop func())) func(pb *testing.PB) {
	return func(pb *testing.PB) {
		push, pop := bind()
		on := false
		for pb.Next() {
			on = !on
			if on {
				push()
			} else {
				pop()
			}
		}
	}
}
