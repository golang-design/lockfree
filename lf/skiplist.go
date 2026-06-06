// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import (
	"math/rand/v2"
	"sync/atomic"

	"golang.design/x/lockfree"
)

// skiplistMaxLevel is the highest level index a node may occupy; the list
// therefore has skiplistMaxLevel+1 levels and supports on the order of 2^32
// elements with the p=0.5 level distribution used by randomLevel.
const skiplistMaxLevel = 31

// SkipList is a lock-free ordered map keyed by K with values V.
//
// It implements the lock-free skip list of Herlihy & Shavit ("The Art of
// Multiprocessor Programming"), the variant built on Harris-style marked
// next-pointers at every level, NOT the optimistic per-node-lock variant.
//
// Progress guarantee: lock-free. Insert (Set) and delete (Del) are CAS loops
// over marked references; on contention they help unlink logically-deleted
// nodes and retry against freshly traversed predecessors/successors, so no
// operation waits on another's completion. Get and Search are wait-free
// traversals. Memory reclamation relies on Go's garbage collector, which keeps
// a node alive while any goroutine still references it.
//
// Len and Range are weakly consistent under concurrency: Len is an approximate
// count, and Range observes a valid-but-not-necessarily-atomic snapshot of the
// keys in [from, to).
//
// (Implementation note: the per-level markable reference is emulated with a
// small heap-allocated wrapper behind an atomic.Pointer for clarity; pointer
// tagging would remove that allocation and is a possible future optimization.)
type SkipList[K, V any] struct {
	head     *skipnode[K, V]
	maxLevel int
	length   atomic.Uint64
	lessFn   lockfree.Less[K]
}

// skipnode is a skip list node. kind is -1 for the head sentinel, +1 for the
// tail sentinel, and 0 for a normal node holding a key/value.
type skipnode[K, V any] struct {
	key      K
	val      atomic.Pointer[V]
	topLevel int
	kind     int8
	nexts    []*markableRef[K, V]
}

// markableRef emulates a java.util.concurrent AtomicMarkableReference: an
// atomically updatable (pointer, mark) pair. The mark indicates that the owning
// node has been logically deleted.
type markableRef[K, V any] struct {
	p atomic.Pointer[markable[K, V]]
}

type markable[K, V any] struct {
	ref    *skipnode[K, V]
	marked bool
}

func newMarkableRef[K, V any](ref *skipnode[K, V], marked bool) *markableRef[K, V] {
	m := &markableRef[K, V]{}
	m.p.Store(&markable[K, V]{ref: ref, marked: marked})
	return m
}

func (m *markableRef[K, V]) get() (ref *skipnode[K, V], marked bool) {
	cur := m.p.Load()
	return cur.ref, cur.marked
}

func (m *markableRef[K, V]) getReference() *skipnode[K, V] {
	return m.p.Load().ref
}

func (m *markableRef[K, V]) set(ref *skipnode[K, V], marked bool) {
	m.p.Store(&markable[K, V]{ref: ref, marked: marked})
}

func (m *markableRef[K, V]) compareAndSet(expectRef, newRef *skipnode[K, V], expectMark, newMark bool) bool {
	cur := m.p.Load()
	if cur.ref != expectRef || cur.marked != expectMark {
		return false
	}
	if newRef == cur.ref && newMark == cur.marked {
		return true // already in the desired state
	}
	return m.p.CompareAndSwap(cur, &markable[K, V]{ref: newRef, marked: newMark})
}

func (m *markableRef[K, V]) attemptMark(expectRef *skipnode[K, V], newMark bool) bool {
	cur := m.p.Load()
	if cur.ref != expectRef {
		return false
	}
	if cur.marked == newMark {
		return true
	}
	return m.p.CompareAndSwap(cur, &markable[K, V]{ref: expectRef, marked: newMark})
}

// NewSkipList returns an empty lock-free skip list ordered by less.
func NewSkipList[K, V any](less lockfree.Less[K]) *SkipList[K, V] {
	maxLevel := skiplistMaxLevel
	tail := &skipnode[K, V]{kind: 1, topLevel: maxLevel}
	tail.nexts = make([]*markableRef[K, V], maxLevel+1)
	for i := range tail.nexts {
		tail.nexts[i] = newMarkableRef[K, V](nil, false)
	}
	head := &skipnode[K, V]{kind: -1, topLevel: maxLevel}
	head.nexts = make([]*markableRef[K, V], maxLevel+1)
	for i := range head.nexts {
		head.nexts[i] = newMarkableRef(tail, false)
	}
	return &SkipList[K, V]{head: head, maxLevel: maxLevel, lessFn: less}
}

func newSkipnode[K, V any](k K, v V, topLevel int) *skipnode[K, V] {
	n := &skipnode[K, V]{key: k, topLevel: topLevel}
	n.val.Store(&v)
	n.nexts = make([]*markableRef[K, V], topLevel+1)
	for i := range n.nexts {
		n.nexts[i] = newMarkableRef[K, V](nil, false)
	}
	return n
}

// keyLess reports whether node n is ordered strictly before key x. The head
// sentinel is treated as -inf and the tail sentinel as +inf.
func (s *SkipList[K, V]) keyLess(n *skipnode[K, V], x K) bool {
	switch n.kind {
	case -1:
		return true
	case 1:
		return false
	default:
		return s.lessFn(n.key, x)
	}
}

// keyEqual reports whether node n holds key x.
func (s *SkipList[K, V]) keyEqual(n *skipnode[K, V], x K) bool {
	if n.kind != 0 {
		return false
	}
	return !s.lessFn(n.key, x) && !s.lessFn(x, n.key)
}

// randomLevel returns a level in [0, max] from a geometric distribution (p=0.5).
func randomLevel(max int) int {
	lvl := 0
	for lvl < max && rand.Uint64()&1 == 0 {
		lvl++
	}
	return lvl
}

// find locates key x, filling preds[level]/succs[level] with the predecessor and
// successor at each level, physically unlinking any logically-deleted nodes it
// encounters. It returns whether x is present.
func (s *SkipList[K, V]) find(x K, preds, succs []*skipnode[K, V]) bool {
retry:
	for {
		pred := s.head
		for level := s.maxLevel; level >= 0; level-- {
			curr := pred.nexts[level].getReference()
			for {
				succ, marked := curr.nexts[level].get()
				for marked { // curr is logically deleted; try to unlink it
					if !pred.nexts[level].compareAndSet(curr, succ, false, false) {
						continue retry
					}
					curr = pred.nexts[level].getReference()
					succ, marked = curr.nexts[level].get()
				}
				if s.keyLess(curr, x) {
					pred = curr
					curr = succ
				} else {
					break
				}
			}
			preds[level] = pred
			succs[level] = curr
		}
		return s.keyEqual(succs[0], x)
	}
}

// Set inserts key k with value v, or updates the value if k already exists.
//
// Updating the value of a key that is being deleted concurrently is weakly
// consistent: the update may be lost if the delete wins the race. This is
// ordinary concurrent-map semantics, not structural corruption.
func (s *SkipList[K, V]) Set(k K, v V) {
	topLevel := randomLevel(s.maxLevel)
	preds := make([]*skipnode[K, V], s.maxLevel+1)
	succs := make([]*skipnode[K, V], s.maxLevel+1)
	for {
		if s.find(k, preds, succs) { // key exists: update in place
			succs[0].val.Store(&v)
			return
		}
		n := newSkipnode(k, v, topLevel)
		for level := 0; level <= topLevel; level++ {
			n.nexts[level].set(succs[level], false)
		}
		pred, succ := preds[0], succs[0]
		if !pred.nexts[0].compareAndSet(succ, n, false, false) {
			continue // someone changed level 0; retry
		}
		for level := 1; level <= topLevel; level++ {
			for {
				pred, succ = preds[level], succs[level]
				if pred.nexts[level].compareAndSet(succ, n, false, false) {
					break
				}
				s.find(k, preds, succs) // refresh preds/succs and retry
			}
		}
		s.length.Add(1)
		return
	}
}

// Del removes key k, returning its value and true if it was present.
func (s *SkipList[K, V]) Del(k K) (v V, ok bool) {
	preds := make([]*skipnode[K, V], s.maxLevel+1)
	succs := make([]*skipnode[K, V], s.maxLevel+1)
	for {
		if !s.find(k, preds, succs) {
			return v, false
		}
		victim := succs[0]
		// Logically delete: mark next-pointers top-down, leaving level 0 last.
		for level := victim.topLevel; level >= 1; level-- {
			succ, marked := victim.nexts[level].get()
			for !marked {
				victim.nexts[level].attemptMark(succ, true)
				succ, marked = victim.nexts[level].get()
			}
		}
		succ, _ := victim.nexts[0].get()
		for {
			iMarkedIt := victim.nexts[0].compareAndSet(succ, succ, false, true)
			var marked bool
			succ, marked = victim.nexts[0].get()
			if iMarkedIt {
				s.find(k, preds, succs) // help physically unlink
				s.length.Add(^uint64(0))
				return *victim.val.Load(), true
			} else if marked {
				return v, false // someone else removed it first
			}
		}
	}
}

// findNode is the wait-free traversal used by Get and Search. It returns the
// live node holding k, or nil if absent.
func (s *SkipList[K, V]) findNode(k K) *skipnode[K, V] {
	pred := s.head
	var curr, succ *skipnode[K, V]
	var marked bool
	for level := s.maxLevel; level >= 0; level-- {
		curr = pred.nexts[level].getReference()
		for {
			succ, marked = curr.nexts[level].get()
			for marked { // skip logically-deleted nodes
				curr = succ
				succ, marked = curr.nexts[level].get()
			}
			if s.keyLess(curr, k) {
				pred = curr
				curr = succ
			} else {
				break
			}
		}
	}
	if s.keyEqual(curr, k) {
		return curr
	}
	return nil
}

// Get returns the value stored for k and whether it was found.
func (s *SkipList[K, V]) Get(k K) (v V, ok bool) {
	n := s.findNode(k)
	if n == nil {
		return v, false
	}
	return *n.val.Load(), true
}

// Search reports whether k is present.
func (s *SkipList[K, V]) Search(k K) bool {
	return s.findNode(k) != nil
}

// Len returns the approximate number of elements in the list.
func (s *SkipList[K, V]) Len() int {
	return int(s.length.Load())
}

// Range calls op for every key/value with from <= key < to, in ascending order.
// It observes a weakly-consistent snapshot under concurrent mutation.
func (s *SkipList[K, V]) Range(from, to K, op func(k K, v V)) {
	// Descend to the first node with key >= from.
	pred := s.head
	var curr, succ *skipnode[K, V]
	var marked bool
	for level := s.maxLevel; level >= 0; level-- {
		curr = pred.nexts[level].getReference()
		for {
			succ, marked = curr.nexts[level].get()
			for marked {
				curr = succ
				succ, marked = curr.nexts[level].get()
			}
			if s.keyLess(curr, from) {
				pred = curr
				curr = succ
			} else {
				break
			}
		}
	}
	for curr.kind == 0 {
		succ, marked = curr.nexts[0].get()
		if marked { // skip a concurrently-deleted node
			curr = succ
			continue
		}
		if !s.lessFn(curr.key, to) { // curr.key >= to: done
			return
		}
		op(curr.key, *curr.val.Load())
		curr = succ
	}
}
