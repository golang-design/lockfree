// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import "sync/atomic"

// Stack is a lock-free LIFO stack (Treiber stack).
//
// Progress guarantee: lock-free. Push and Pop are CAS loops over the top
// pointer; on a failed CAS each retries against the freshly loaded top, and no
// operation ever waits on another's completion. Go's garbage collector keeps a
// popped node alive while any goroutine still references it, which is what makes
// the bare compare-and-swap safe from use-after-free here.
type Stack[T any] struct {
	top atomic.Pointer[directItem[T]]
	len atomic.Uint64
}

// NewStack creates a new lock-free stack.
func NewStack[T any]() *Stack[T] {
	return &Stack[T]{}
}

// Pop removes and returns the value at the top of the stack.
// The second return value is false if the stack is empty.
func (s *Stack[T]) Pop() (v T, ok bool) {
	for {
		top := s.top.Load()
		if top == nil {
			return v, false
		}
		next := top.next.Load()
		if s.top.CompareAndSwap(top, next) {
			s.len.Add(^uint64(0))
			return top.v, true
		}
	}
}

// Push pushes a value on top of the stack.
func (s *Stack[T]) Push(v T) {
	item := &directItem[T]{v: v}
	for {
		top := s.top.Load()
		item.next.Store(top)
		if s.top.CompareAndSwap(top, item) {
			s.len.Add(1)
			return
		}
	}
}

// Length returns the number of elements currently in the stack.
func (s *Stack[T]) Length() uint64 {
	return s.len.Load()
}
