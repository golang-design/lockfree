// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf

import (
	"sync/atomic"

	"golang.design/x/lockfree"
)

// The wait-free map is built with Herlihy's wait-free universal construction
// (Herlihy, "Wait-Free Synchronization", ACM TOPLAS 1991; Herlihy & Shavit,
// "The Art of Multiprocessor Programming", section 6.4), the same construction
// used by the wait-free Stack and Deque here: operations are linearized into a
// single consensus-ordered list and every participant helps append announced
// operations in round-robin order, so no operation can be starved.
//
// The sequential object is an ordered map represented persistently as an
// immutable, height-balanced (AVL) binary search tree keyed by K (Okasaki,
// "Purely Functional Data Structures", 1998, for the persistent-structure
// approach). Applying one operation copies only the O(log n) nodes on the path
// to the affected key and shares the rest, so each operation node stores the
// resulting tree and its response as a pure function of its predecessor's tree.
//
// Every operation, including the read-only Get, is linearized through the
// construction, so each is wait-free and linearizable at O(maxHandles * log n):
// up to O(maxHandles) helping rounds, each of which may apply one O(log n) tree
// operation. This is the documented cost of a universal-construction map; it is
// size-dependent, unlike the O(maxHandles) wait-free Queue and Stack.

type mapOp int8

const (
	mapSet mapOp = iota
	mapGet
	mapDel
)

// mapTreeNode is one node of an immutable, key-ordered AVL tree. Nodes are
// shared across versions and never mutated after construction.
type mapTreeNode[K, V any] struct {
	key         K
	val         V
	left, right *mapTreeNode[K, V]
	height      int
}

func mapHeight[K, V any](n *mapTreeNode[K, V]) int {
	if n == nil {
		return 0
	}
	return n.height
}

// mkMapNode builds a fresh node from key/val and the two subtrees.
func mkMapNode[K, V any](key K, val V, left, right *mapTreeNode[K, V]) *mapTreeNode[K, V] {
	h := mapHeight(left)
	if rh := mapHeight(right); rh > h {
		h = rh
	}
	return &mapTreeNode[K, V]{key: key, val: val, left: left, right: right, height: h + 1}
}

func rotateMapRight[K, V any](n *mapTreeNode[K, V]) *mapTreeNode[K, V] {
	l := n.left
	return mkMapNode(l.key, l.val, l.left, mkMapNode(n.key, n.val, l.right, n.right))
}

func rotateMapLeft[K, V any](n *mapTreeNode[K, V]) *mapTreeNode[K, V] {
	r := n.right
	return mkMapNode(r.key, r.val, mkMapNode(n.key, n.val, n.left, r.left), r.right)
}

// rebalanceMap restores the AVL invariant at n after one subtree changed height
// by at most one.
func rebalanceMap[K, V any](n *mapTreeNode[K, V]) *mapTreeNode[K, V] {
	switch bf := mapHeight(n.left) - mapHeight(n.right); {
	case bf > 1:
		if mapHeight(n.left.left) >= mapHeight(n.left.right) {
			return rotateMapRight(n)
		}
		return rotateMapRight(mkMapNode(n.key, n.val, rotateMapLeft(n.left), n.right))
	case bf < -1:
		if mapHeight(n.right.right) >= mapHeight(n.right.left) {
			return rotateMapLeft(n)
		}
		return rotateMapLeft(mkMapNode(n.key, n.val, n.left, rotateMapRight(n.right)))
	default:
		return n
	}
}

// mapInsert returns n with key set to val, replacing any existing value.
func mapInsert[K, V any](n *mapTreeNode[K, V], key K, val V, less lockfree.Less[K]) *mapTreeNode[K, V] {
	if n == nil {
		return mkMapNode(key, val, nil, nil)
	}
	switch {
	case less(key, n.key):
		return rebalanceMap(mkMapNode(n.key, n.val, mapInsert(n.left, key, val, less), n.right))
	case less(n.key, key):
		return rebalanceMap(mkMapNode(n.key, n.val, n.left, mapInsert(n.right, key, val, less)))
	default:
		return mkMapNode(n.key, val, n.left, n.right) // key exists: replace value
	}
}

// mapLookup returns the value stored under key and whether it was present.
func mapLookup[K, V any](n *mapTreeNode[K, V], key K, less lockfree.Less[K]) (V, bool) {
	for n != nil {
		switch {
		case less(key, n.key):
			n = n.left
		case less(n.key, key):
			n = n.right
		default:
			return n.val, true
		}
	}
	var zero V
	return zero, false
}

// mapMin returns the key/value of the leftmost (smallest-key) node.
func mapMin[K, V any](n *mapTreeNode[K, V]) (K, V) {
	for n.left != nil {
		n = n.left
	}
	return n.key, n.val
}

// mapDelete removes key, returning the new tree, the removed value, and whether
// it was present. A node with two children is replaced by its in-order successor
// (the smallest key in its right subtree).
func mapDelete[K, V any](n *mapTreeNode[K, V], key K, less lockfree.Less[K]) (*mapTreeNode[K, V], V, bool) {
	if n == nil {
		var zero V
		return nil, zero, false
	}
	switch {
	case less(key, n.key):
		nl, old, ok := mapDelete(n.left, key, less)
		return rebalanceMap(mkMapNode(n.key, n.val, nl, n.right)), old, ok
	case less(n.key, key):
		nr, old, ok := mapDelete(n.right, key, less)
		return rebalanceMap(mkMapNode(n.key, n.val, n.left, nr)), old, ok
	default:
		old := n.val
		if n.left == nil {
			return n.right, old, true
		}
		if n.right == nil {
			return n.left, old, true
		}
		sk, sv := mapMin(n.right)
		nr, _, _ := mapDelete(n.right, sk, less)
		return rebalanceMap(mkMapNode(sk, sv, n.left, nr)), old, true
	}
}

// mapResult is the state and response produced by applying one operation: root
// is the resulting tree, and (val, ok) is the Get/Del response.
type mapResult[K, V any] struct {
	root *mapTreeNode[K, V]
	val  V
	ok   bool
}

// mapNode is an entry in the consensus-ordered operation list. res is always
// published before seq, so a non-zero seq guarantees res is visible.
type mapNode[K, V any] struct {
	op   mapOp
	key  K
	val  V
	next atomic.Pointer[mapNode[K, V]]
	res  atomic.Pointer[mapResult[K, V]]
	seq  atomic.Int64
}

// Map is a wait-free ordered key/value map for multiple concurrent participants,
// built on the wait-free universal construction.
//
// Progress guarantee: wait-free. Each operation announces itself, then every
// participant helps append announced operations in round-robin order, so any
// announced operation is linked within a bounded number of steps (O(maxHandles))
// regardless of scheduling. There are no locks and no operation waits on
// another's completion. Each operation costs O(maxHandles * log n): up to
// O(maxHandles) helping rounds, each of which may apply one O(log n) tree
// operation, where n is the number of entries. Get is linearized through the
// construction too, so reads carry the same cost as writes.
//
// Memory reclamation relies on Go's garbage collector: the per-participant
// cursors and an anchor advance toward the frontier, so once every participant
// has moved past an operation node it (and the persistent-tree versions it alone
// kept alive) becomes unreachable and is collected.
//
// Like the wait-free Queue, Stack, and Deque, participants must register: obtain
// one Handle per goroutine (a MapHandle) up to maxHandles. Slots are not
// reclaimable (maxHandles is the lifetime total), which suits a bounded,
// long-lived worker pool.
type Map[K, V any] struct {
	anchor   atomic.Pointer[mapNode[K, V]]
	head     []atomic.Pointer[mapNode[K, V]]
	announce []atomic.Pointer[mapNode[K, V]]
	cursor   atomic.Int64
	n        int
	less     lockfree.Less[K]
}

// NewMap creates a wait-free ordered map keyed by K and ordered by less.
// maxHandles is the total number of Handle registrations allowed over the map's
// lifetime (slots are not reclaimable); it must be at least 1.
func NewMap[K, V any](maxHandles int, less lockfree.Less[K]) *Map[K, V] {
	if maxHandles < 1 {
		panic("wf: NewMap maxHandles must be >= 1")
	}
	sentinel := &mapNode[K, V]{}
	sentinel.res.Store(&mapResult[K, V]{}) // empty map
	sentinel.seq.Store(1)
	m := &Map[K, V]{
		head:     make([]atomic.Pointer[mapNode[K, V]], maxHandles),
		announce: make([]atomic.Pointer[mapNode[K, V]], maxHandles),
		n:        maxHandles,
		less:     less,
	}
	m.anchor.Store(sentinel)
	return m
}

// MapHandle is a participant's access point to a Map. It is bound to one slot in
// the helping arrays and is NOT safe for concurrent use by multiple goroutines:
// acquire one per goroutine.
type MapHandle[K, V any] struct {
	m   *Map[K, V]
	tid int
}

// Handle registers a new participant and returns its MapHandle. Each call
// permanently consumes one of the maxHandles slots. It panics if more than
// maxHandles handles are requested.
func (m *Map[K, V]) Handle() *MapHandle[K, V] {
	id := m.cursor.Add(1) - 1
	if id >= int64(m.n) {
		panic("wf: too many handles (exceeds maxHandles passed to NewMap)")
	}
	frontier := m.anchor.Load()
	m.head[id].Store(frontier)
	m.announce[id].Store(frontier)
	return &MapHandle[K, V]{m: m, tid: int(id)}
}

// Set stores value v under key k, replacing any existing value.
func (h *MapHandle[K, V]) Set(k K, v V) { h.m.apply(h.tid, mapSet, k, v) }

// Get returns the value stored under key k and whether it was found.
func (h *MapHandle[K, V]) Get(k K) (V, bool) {
	var zero V
	r := h.m.apply(h.tid, mapGet, k, zero)
	return r.val, r.ok
}

// Del removes key k, returning its value and true if it was present.
func (h *MapHandle[K, V]) Del(k K) (V, bool) {
	var zero V
	r := h.m.apply(h.tid, mapDel, k, zero)
	return r.val, r.ok
}

// maxFrontier returns the furthest known operation node (largest seq).
func (m *Map[K, V]) maxFrontier() *mapNode[K, V] {
	best := m.anchor.Load()
	bestSeq := best.seq.Load()
	for i := 0; i < m.n; i++ {
		if nd := m.head[i].Load(); nd != nil {
			if sq := nd.seq.Load(); sq > bestSeq {
				best, bestSeq = nd, sq
			}
		}
	}
	return best
}

func (m *Map[K, V]) apply(tid int, op mapOp, key K, val V) *mapResult[K, V] {
	mine := &mapNode[K, V]{op: op, key: key, val: val}
	m.announce[tid].Store(mine)
	m.head[tid].Store(m.maxFrontier())

	for mine.seq.Load() == 0 {
		before := m.head[tid].Load()
		var prefer *mapNode[K, V]
		helpIdx := (before.seq.Load() + 1) % int64(m.n)
		if cand := m.announce[helpIdx].Load(); cand != nil && cand.seq.Load() == 0 {
			prefer = cand
		} else {
			prefer = mine
		}
		before.next.CompareAndSwap(nil, prefer)
		after := before.next.Load()
		if after.seq.Load() == 0 {
			m.publish(before, after)
		}
		m.head[tid].Store(after)
	}

	r := mine.res.Load()
	m.head[tid].Store(mine)
	m.advanceAnchor(mine)
	return r
}

// publish computes after's result from before's result and publishes it, then
// its sequence number. res is set before seq, and both stores are idempotent so
// every helper computes the same values.
func (m *Map[K, V]) publish(before, after *mapNode[K, V]) {
	br := before.res.Load()
	var nr *mapResult[K, V]
	switch after.op {
	case mapSet:
		nr = &mapResult[K, V]{root: mapInsert(br.root, after.key, after.val, m.less)}
	case mapGet:
		v, ok := mapLookup(br.root, after.key, m.less)
		nr = &mapResult[K, V]{root: br.root, val: v, ok: ok}
	default: // mapDel
		root, v, ok := mapDelete(br.root, after.key, m.less)
		nr = &mapResult[K, V]{root: root, val: v, ok: ok}
	}
	after.res.CompareAndSwap(nil, nr)
	after.seq.CompareAndSwap(0, before.seq.Load()+1)
}

// advanceAnchor moves the anchor forward to n if n is further along, letting the
// garbage collector reclaim the now-unreachable prefix of the operation list.
func (m *Map[K, V]) advanceAnchor(n *mapNode[K, V]) {
	for {
		cur := m.anchor.Load()
		if n.seq.Load() <= cur.seq.Load() {
			return
		}
		if m.anchor.CompareAndSwap(cur, n) {
			return
		}
	}
}
