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

// Compile-time assertions that the lock-free types satisfy the shared contracts.
var (
	_ lockfree.Queue[int]    = (*lf.Queue[int])(nil)
	_ lockfree.Stack[int]    = (*lf.Stack[int])(nil)
	_ lockfree.Map[int, int] = (*lf.SkipList[int, int])(nil)
	_ lockfree.Map[int, int] = (*lf.OrderedMap[int, int])(nil)
)

// TestQueueConformance runs the shared FIFO-queue conformance suite against the
// lock-free Michael & Scott queue. A lock-free queue has unbounded participants,
// so every goroutine shares the same queue value.
func TestQueueConformance(t *testing.T) {
	conformtest.Queue(t, func(maxParticipants int) func() lockfree.Queue[int] {
		q := lf.NewQueue[int]()
		return func() lockfree.Queue[int] { return q }
	})
}

// TestStackConformance runs the shared LIFO-stack conformance suite against the
// lock-free Treiber stack.
func TestStackConformance(t *testing.T) {
	conformtest.Stack(t, func(maxParticipants int) func() lockfree.Stack[int] {
		s := lf.NewStack[int]()
		return func() lockfree.Stack[int] { return s }
	})
}

// TestMapConformance runs the shared map conformance suite against both
// lock-free map implementations: the skip list and the ordered-map facade over
// it.
func TestMapConformance(t *testing.T) {
	t.Run("SkipList", func(t *testing.T) {
		conformtest.Map(t, func() lockfree.Map[int, int] {
			return lf.NewSkipList[int, int](func(a, b int) bool { return a < b })
		})
	})
	t.Run("OrderedMap", func(t *testing.T) {
		conformtest.Map(t, func() lockfree.Map[int, int] {
			return lf.NewOrderedMap[int, int](func(a, b int) bool { return a < b })
		})
	})
}
