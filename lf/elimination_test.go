// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree/lf"
)

// Behavior is covered by the shared Stack conformance suite (conformance_test.go)
// and the elimination mechanism is checked by the white-box TestEliminationFires.

// BenchmarkStackContended compares the plain Treiber stack with the
// elimination-backoff stack under a balanced, high-contention push/pop workload,
// where elimination is expected to help.
func BenchmarkStackContended(b *testing.B) {
	impls := []struct {
		name string
		make func() stackInterface
	}{
		{"treiber", func() stackInterface { return lf.NewStack[int]() }},
		{"elimination", func() stackInterface { return lf.NewEliminationStack[int]() }},
	}
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			s := impl.make()
			var c int64
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if atomic.AddInt64(&c, 1)&1 == 0 {
						s.Push(1)
					} else {
						s.Pop()
					}
				}
			})
		})
	}
}
