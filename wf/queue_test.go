// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"testing"

	"golang.design/x/lockfree/wf"
)

// FIFO, empty, and concurrent behavior are covered by the shared conformance
// suite (see conformance_test.go). The tests here cover wf-specific behavior:
// the bounded, non-reclaimable Handle budget.

func TestQueueHandleExhaustion(t *testing.T) {
	q := wf.NewQueue[int](2)
	q.Handle()
	q.Handle()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when exceeding maxHandles")
		}
	}()
	q.Handle() // third handle exceeds maxHandles=2
}

func TestNewQueueInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for maxHandles < 1")
		}
	}()
	wf.NewQueue[int](0)
}
