// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf_test

import (
	"testing"

	"golang.design/x/lockfree"
	"golang.design/x/lockfree/internal/conformtest"
	"golang.design/x/lockfree/lf"
)

// TestDequeConformance runs the shared deque conformance suite (sequential over
// all four operations, a both-ends differential fuzz, and concurrent
// conservation) against the lock-free Sundell & Tsigas deque.
func TestDequeConformance(t *testing.T) {
	conformtest.Deque(t, func(maxParticipants int) func() lockfree.Deque[int] {
		d := lf.NewDeque[int]()
		return func() lockfree.Deque[int] { return d }
	})
}
