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

// Compile-time assertions. Note it is the per-goroutine handle, not the Queue or
// Stack value, that satisfies the shared contract.
var (
	_ lockfree.Queue[int] = (*wf.Handle[int])(nil)
	_ lockfree.Stack[int] = (*wf.StackHandle[int])(nil)
)

// TestQueueConformance runs the shared FIFO-queue conformance suite against the
// wait-free Kogan & Petrank queue. Participants are bounded, so each goroutine
// gets its own Handle from the maxParticipants budget.
func TestQueueConformance(t *testing.T) {
	conformtest.Queue(t, func(maxParticipants int) func() lockfree.Queue[int] {
		q := wf.NewQueue[int](maxParticipants)
		return func() lockfree.Queue[int] { return q.Handle() }
	})
}

// TestStackConformance runs the shared LIFO-stack conformance suite against the
// wait-free stack (Herlihy's universal construction).
func TestStackConformance(t *testing.T) {
	conformtest.Stack(t, func(maxParticipants int) func() lockfree.Stack[int] {
		s := wf.NewStack[int](maxParticipants)
		return func() lockfree.Stack[int] { return s.Handle() }
	})
}
