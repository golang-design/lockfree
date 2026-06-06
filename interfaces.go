// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree

// These interfaces are the guarantee-neutral contracts for the abstract data
// types implemented in the lf (lock-free) and wf (wait-free) subpackages. They
// let callers program against an ADT and choose the progress guarantee by
// constructor, and they let a single behavioral conformance suite verify every
// implementation of the same ADT.
//
// Note the wait-free side is not always a symmetric drop-in: a wait-free MPMC
// queue requires participants to register, so it is wf's per-goroutine Handle
// (not the wf.Queue value itself) that satisfies Queue. See package wf.

// Queue is a first-in-first-out queue. Satisfied by *lf.Queue[T] and by
// *wf.Handle[T].
type Queue[T any] interface {
	Enqueue(v T)
	Dequeue() (v T, ok bool)
}

// Stack is a last-in-first-out stack. Satisfied by *lf.Stack[T].
type Stack[T any] interface {
	Push(v T)
	Pop() (v T, ok bool)
}

// Map is a key/value map. Satisfied by *lf.SkipList[K, V].
type Map[K, V any] interface {
	Set(k K, v V)
	Get(k K) (v V, ok bool)
	Del(k K) (v V, ok bool)
}
