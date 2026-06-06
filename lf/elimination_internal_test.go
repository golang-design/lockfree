// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import (
	"sync"
	"testing"
)

// TestEliminationFires is a white-box test guarding the elimination mechanism's
// most insidious failure: a broken exchanger that never matches would still pass
// every behavioral and conservation test, because every operation falls back to
// the central Treiber stack. It runs a balanced push/pop workload with few slots
// and asserts the internal successful-elimination counter is non-zero, i.e. the
// elimination path actually executes.
func TestEliminationFires(t *testing.T) {
	s := newEliminationStack[int](4, 256) // few slots + generous spins encourage matches
	const workers = 8
	const ops = 50000

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				if id%2 == 0 {
					s.Push(i)
				} else {
					s.Pop()
				}
			}
		}(w)
	}
	wg.Wait()

	if got := s.eliminations.Load(); got == 0 {
		t.Fatal("elimination never fired: the elimination mechanism is dead code")
	} else {
		t.Logf("successful eliminations: %d", got)
	}
}
