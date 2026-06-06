// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

// Package lockfree offers concurrent data structures with non-blocking
// progress guarantees.
//
// Each type documents its precise guarantee rather than a blanket "lock-free"
// claim, because they differ:
//
//   - Stack[T]       lock-free LIFO (Treiber)
//   - Queue[T]       lock-free FIFO (Michael & Scott)
//   - RingBuffer[T]  wait-free bounded SPSC ring buffer (single producer,
//     single consumer)
//   - SkipList[K,V]  lock-free ordered map (Herlihy & Shavit, marked pointers);
//     Get/Search are wait-free
//   - OrderedMap[K,V] lock-free ordered map backed by SkipList
//   - AddFloat64     lock-free atomic float64 addition
//
// Progress guarantees here mean: wait-free — every operation completes in a
// bounded number of its own steps; lock-free — some operation always makes
// progress system-wide, with no locks and no operation waiting on another.
// The race detector verifies memory safety but cannot prove these guarantees;
// the argument for each lives in that type's doc comment.
//
// Memory reclamation relies on Go's garbage collector: a node stays alive while
// any goroutine still references it, which is what makes the bare
// compare-and-swap loops safe from use-after-free and the classic ABA hazard.
//
// Note that this package is under development and not for production use.
package lockfree // import "golang.design/x/lockfree"
