// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"testing"

	"golang.design/x/lockfree/wf"
)

func TestQueueEmpty(t *testing.T) {
	q := wf.NewQueue[int](1)
	h := q.Handle()
	if v, ok := h.Dequeue(); ok {
		t.Fatalf("dequeue empty queue returned ok, got %v", v)
	}
}

func TestQueueFIFO(t *testing.T) {
	q := wf.NewQueue[int](1)
	h := q.Handle()
	for i := 0; i < 100; i++ {
		h.Enqueue(i)
	}
	for i := 0; i < 100; i++ {
		v, ok := h.Dequeue()
		if !ok || v != i {
			t.Fatalf("dequeue: got (%v,%v), want (%d,true)", v, ok, i)
		}
	}
	if v, ok := h.Dequeue(); ok {
		t.Fatalf("dequeue drained queue returned ok, got %v", v)
	}
}

func TestQueueInterleaved(t *testing.T) {
	q := wf.NewQueue[int](1)
	h := q.Handle()
	h.Enqueue(1)
	h.Enqueue(2)
	if v, ok := h.Dequeue(); !ok || v != 1 {
		t.Fatalf("got (%v,%v), want (1,true)", v, ok)
	}
	h.Enqueue(3)
	for _, want := range []int{2, 3} {
		if v, ok := h.Dequeue(); !ok || v != want {
			t.Fatalf("got (%v,%v), want (%d,true)", v, ok, want)
		}
	}
	if _, ok := h.Dequeue(); ok {
		t.Fatalf("expected empty")
	}
}

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
