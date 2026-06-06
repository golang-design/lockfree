// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import (
	"sync/atomic"

	"golang.design/x/lockfree"
)

// List is a lock-free ordered list-based map keyed by K with values V
// (Harris 2001 / Michael 2002). It is the building block for the lock-free hash
// table and is also a self-contained ordered map; lookups are O(n), so prefer
// SkipList for large ordered maps and HashMap for large unordered ones.
//
// Progress guarantee: lock-free. Set and Del are CAS loops that help physically
// unlink logically-deleted (marked) nodes and retry against a freshly traversed
// predecessor/successor pair, so no operation waits on another's completion. Get
// and Contains are wait-free traversals. Memory reclamation relies on Go's
// garbage collector, which keeps a node alive while any goroutine still
// references it.
//
// Len is an approximate count under concurrency; Range observes a
// weakly-consistent snapshot.
type List[K, V any] struct {
	head   *listNode[K, V]
	length atomic.Uint64
	lessFn lockfree.Less[K]
}

// listNode is a list element. kind is -1 for the head sentinel, +1 for the tail
// sentinel, and 0 for a normal node. next is a markable reference: a set mark
// means this node has been logically deleted.
type listNode[K, V any] struct {
	key  K
	val  atomic.Pointer[V]
	kind int8
	next listMarkableRef[K, V]
}

// listMarkableRef is an AtomicMarkableReference for list nodes: an atomically
// updatable (pointer, mark) pair where the mark flags logical deletion. It
// mirrors the skip list's markableRef but for listNode.
type listMarkableRef[K, V any] struct {
	p atomic.Pointer[listMarkable[K, V]]
}

type listMarkable[K, V any] struct {
	ref    *listNode[K, V]
	marked bool
}

func (m *listMarkableRef[K, V]) get() (ref *listNode[K, V], marked bool) {
	cur := m.p.Load()
	return cur.ref, cur.marked
}

func (m *listMarkableRef[K, V]) getReference() *listNode[K, V] {
	return m.p.Load().ref
}

func (m *listMarkableRef[K, V]) set(ref *listNode[K, V], marked bool) {
	m.p.Store(&listMarkable[K, V]{ref: ref, marked: marked})
}

func (m *listMarkableRef[K, V]) compareAndSet(expectRef, newRef *listNode[K, V], expectMark, newMark bool) bool {
	cur := m.p.Load()
	if cur.ref != expectRef || cur.marked != expectMark {
		return false
	}
	if newRef == cur.ref && newMark == cur.marked {
		return true
	}
	return m.p.CompareAndSwap(cur, &listMarkable[K, V]{ref: newRef, marked: newMark})
}

// NewList returns an empty lock-free ordered list ordered by less.
func NewList[K, V any](less lockfree.Less[K]) *List[K, V] {
	tail := &listNode[K, V]{kind: 1}
	tail.next.set(nil, false) // initialize so tail.next.get() never dereferences nil
	head := &listNode[K, V]{kind: -1}
	head.next.set(tail, false)
	return &List[K, V]{head: head, lessFn: less}
}

func (l *List[K, V]) keyLess(n *listNode[K, V], x K) bool {
	switch n.kind {
	case -1:
		return true
	case 1:
		return false
	default:
		return l.lessFn(n.key, x)
	}
}

func (l *List[K, V]) keyEqual(n *listNode[K, V], x K) bool {
	if n.kind != 0 {
		return false
	}
	return !l.lessFn(n.key, x) && !l.lessFn(x, n.key)
}

// search returns pred and curr such that pred precedes the position of key and
// curr is the first node with key >= x, physically unlinking marked nodes along
// the way.
func (l *List[K, V]) search(x K) (pred, curr *listNode[K, V]) {
retry:
	for {
		pred = l.head
		curr = pred.next.getReference()
		for {
			succ, marked := curr.next.get()
			for marked { // curr is logically deleted; try to unlink it
				if !pred.next.compareAndSet(curr, succ, false, false) {
					continue retry
				}
				curr = succ
				succ, marked = curr.next.get()
			}
			if l.keyLess(curr, x) {
				pred = curr
				curr = succ
			} else {
				return pred, curr
			}
		}
	}
}

// Set inserts key k with value v, or updates the value if k already exists.
//
// Updating the value of a key being deleted concurrently is weakly consistent:
// the update may be lost if the delete wins the race. This is ordinary
// concurrent-map semantics, not structural corruption.
func (l *List[K, V]) Set(k K, v V) {
	for {
		pred, curr := l.search(k)
		if l.keyEqual(curr, k) { // key exists: update in place
			curr.val.Store(&v)
			return
		}
		n := &listNode[K, V]{key: k}
		n.val.Store(&v)
		n.next.set(curr, false)
		if pred.next.compareAndSet(curr, n, false, false) {
			l.length.Add(1)
			return
		}
	}
}

// Get returns the value stored for k and whether it was found.
func (l *List[K, V]) Get(k K) (v V, ok bool) {
	curr := l.head.next.getReference()
	for {
		succ, marked := curr.next.get()
		for marked {
			curr = succ
			succ, marked = curr.next.get()
		}
		if l.keyLess(curr, k) {
			curr = succ
			continue
		}
		if l.keyEqual(curr, k) {
			return *curr.val.Load(), true
		}
		return v, false
	}
}

// Contains reports whether k is present.
func (l *List[K, V]) Contains(k K) bool {
	_, ok := l.Get(k)
	return ok
}

// Del removes key k, returning its value and true if it was present.
func (l *List[K, V]) Del(k K) (v V, ok bool) {
	for {
		pred, curr := l.search(k)
		if !l.keyEqual(curr, k) {
			return v, false
		}
		succ, marked := curr.next.get()
		if marked {
			continue // already being deleted; retry to get a fresh view
		}
		// Only the goroutine that wins the unmarked->marked transition performs
		// the logical deletion (and counts it); a node already marked makes this
		// CAS fail, so the decrement happens exactly once.
		if !curr.next.compareAndSet(succ, succ, false, true) {
			continue
		}
		value := *curr.val.Load()
		pred.next.compareAndSet(curr, succ, false, false) // try to physically unlink
		l.length.Add(^uint64(0))
		return value, true
	}
}

// Len returns the approximate number of elements in the list.
func (l *List[K, V]) Len() int {
	return int(l.length.Load())
}

// Range calls op for every key/value with from <= key < to, in ascending order.
// It observes a weakly-consistent snapshot under concurrent mutation.
func (l *List[K, V]) Range(from, to K, op func(k K, v V)) {
	_, curr := l.search(from)
	for curr.kind == 0 {
		succ, marked := curr.next.get()
		if marked {
			curr = succ
			continue
		}
		if !l.lessFn(curr.key, to) {
			return
		}
		op(curr.key, *curr.val.Load())
		curr = succ
	}
}
