// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"testing"

	"golang.design/x/lockfree"
	"golang.design/x/lockfree/internal/conformtest"
	"golang.design/x/lockfree/wf"
)

// Compile-time assertion that a wait-free Handle satisfies the shared contract.
// Note it is the per-goroutine Handle, not the Queue value, that satisfies it.
var _ lockfree.Queue[int] = (*wf.Handle[int])(nil)

// TestQueueConformance runs the shared FIFO-queue conformance suite against the
// wait-free Kogan & Petrank queue. Participants are bounded, so each goroutine
// gets its own Handle from the maxParticipants budget.
func TestQueueConformance(t *testing.T) {
	conformtest.Queue(t, func(maxParticipants int) func() lockfree.Queue[int] {
		q := wf.NewQueue[int](maxParticipants)
		return func() lockfree.Queue[int] { return q.Handle() }
	})
}
