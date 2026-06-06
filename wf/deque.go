// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf

import "sync/atomic"

// The wait-free deque is built with Herlihy's wait-free universal construction
// (Herlihy, "Wait-Free Synchronization", ACM TOPLAS 1991; Herlihy & Shavit,
// "The Art of Multiprocessor Programming", section 6.4), the same construction
// used by the wait-free Stack here: operations are linearized into a single
// consensus-ordered list and every participant helps append announced operations
// in round-robin order, so no operation can be starved.
//
// The sequential object is a deque represented persistently as an immutable,
// height-balanced (AVL) binary tree whose in-order sequence is the deque from
// front to back (Okasaki, "Purely Functional Data Structures", 1998, for the
// persistent-structure approach). Applying one operation copies only the
// O(log n) nodes on the path from the root to the affected end and shares the
// rest, so each operation node stores the resulting tree and its response as a
// pure function of its predecessor's tree. Each applied operation is therefore
// an O(log n) tree update.
//
// Unlike the wait-free Queue and Stack, whose operations are O(maxHandles) and
// independent of size, this deque is O(maxHandles * log n): the helping loop runs
// up to O(maxHandles) rounds and each round may apply one O(log n) tree update
// (the stack collapses to O(maxHandles) only because its apply is O(1)). The
// log n size dependence appears to be inherent to wait-free deques (the dedicated
// state of the art, Asbell & Ruppert, "A Wait-Free Deque With Polylogarithmic
// Step Complexity", OPODIS 2023, is also polylogarithmic in the size), so it is a
// genuine bound rather than an artifact of the construction.

type dequeOp int8

const (
	dqPushFront dequeOp = iota
	dqPushBack
	dqPopFront
	dqPopBack
)

// treeNode is one node of an immutable AVL tree. Nodes are shared across
// versions and never mutated after construction; the in-order traversal is the
// deque from front (leftmost) to back (rightmost).
type treeNode[T any] struct {
	val         T
	left, right *treeNode[T]
	height      int
}

func height[T any](n *treeNode[T]) int {
	if n == nil {
		return 0
	}
	return n.height
}

// mk builds a fresh node from val and the two subtrees, computing its height.
func mk[T any](val T, left, right *treeNode[T]) *treeNode[T] {
	h := height(left)
	if rh := height(right); rh > h {
		h = rh
	}
	return &treeNode[T]{val: val, left: left, right: right, height: h + 1}
}

// rotateRight lifts n's left child above n (n must have a left child).
func rotateRight[T any](n *treeNode[T]) *treeNode[T] {
	l := n.left
	return mk(l.val, l.left, mk(n.val, l.right, n.right))
}

// rotateLeft lifts n's right child above n (n must have a right child).
func rotateLeft[T any](n *treeNode[T]) *treeNode[T] {
	r := n.right
	return mk(r.val, mk(n.val, n.left, r.left), r.right)
}

// rebalance restores the AVL invariant at n after one of its subtrees changed
// height by at most one, returning the (possibly rotated) subtree.
func rebalance[T any](n *treeNode[T]) *treeNode[T] {
	switch bf := height(n.left) - height(n.right); {
	case bf > 1: // left heavy
		if height(n.left.left) >= height(n.left.right) {
			return rotateRight(n)
		}
		return rotateRight(mk(n.val, rotateLeft(n.left), n.right))
	case bf < -1: // right heavy
		if height(n.right.right) >= height(n.right.left) {
			return rotateLeft(n)
		}
		return rotateLeft(mk(n.val, n.left, rotateRight(n.right)))
	default:
		return n
	}
}

// treePushFront returns n with x inserted as the new leftmost (front) element.
func treePushFront[T any](n *treeNode[T], x T) *treeNode[T] {
	if n == nil {
		return mk(x, nil, nil)
	}
	return rebalance(mk(n.val, treePushFront(n.left, x), n.right))
}

// treePushBack returns n with x inserted as the new rightmost (back) element.
func treePushBack[T any](n *treeNode[T], x T) *treeNode[T] {
	if n == nil {
		return mk(x, nil, nil)
	}
	return rebalance(mk(n.val, n.left, treePushBack(n.right, x)))
}

// treePopFront removes the leftmost (front) element, returning the new tree, the
// removed value, and whether the deque was non-empty.
func treePopFront[T any](n *treeNode[T]) (*treeNode[T], T, bool) {
	if n == nil {
		var zero T
		return nil, zero, false
	}
	if n.left == nil {
		return n.right, n.val, true
	}
	newLeft, v, ok := treePopFront(n.left)
	return rebalance(mk(n.val, newLeft, n.right)), v, ok
}

// treePopBack removes the rightmost (back) element.
func treePopBack[T any](n *treeNode[T]) (*treeNode[T], T, bool) {
	if n == nil {
		var zero T
		return nil, zero, false
	}
	if n.right == nil {
		return n.left, n.val, true
	}
	newRight, v, ok := treePopBack(n.right)
	return rebalance(mk(n.val, n.left, newRight)), v, ok
}

// dequeResult is the state and response produced by applying one operation: root
// is the resulting deque, and (val, ok) is a pop's response.
type dequeResult[T any] struct {
	root *treeNode[T]
	val  T
	ok   bool
}

// dequeNode is an entry in the consensus-ordered operation list. res is always
// published before seq, so a non-zero seq guarantees res is visible.
type dequeNode[T any] struct {
	op   dequeOp
	arg  T
	next atomic.Pointer[dequeNode[T]]   // consensus winner for the successor
	res  atomic.Pointer[dequeResult[T]] // state+response after applying this op
	seq  atomic.Int64                   // position in the list; 0 until linked
}

// Deque is a wait-free double-ended queue for multiple concurrent participants,
// built on the wait-free universal construction.
//
// Progress guarantee: wait-free. Each operation announces itself, then every
// participant helps append announced operations in round-robin order, so any
// announced operation is linked within a bounded number of steps (O(maxHandles))
// regardless of scheduling. There are no locks and no operation waits on
// another's completion. Each operation costs O(maxHandles * log n): up to
// O(maxHandles) helping rounds, each of which may apply one O(log n)
// persistent-tree update, where n is the number of elements.
//
// Memory reclamation relies on Go's garbage collector. The construction keeps no
// permanent reference to the head of the operation list: the per-participant
// cursors and an anchor advance toward the frontier, so once every participant
// has moved past an operation node it (and the persistent-tree versions it alone
// kept alive) becomes unreachable and is collected.
//
// Like the wait-free Queue and Stack, participants must register because the
// helping arrays are indexed by participant id: obtain one Handle per goroutine
// (a DequeHandle) up to maxHandles. Slots are not reclaimable (maxHandles is the
// lifetime total), which suits a bounded, long-lived worker pool.
type Deque[T any] struct {
	anchor   atomic.Pointer[dequeNode[T]]   // a recent node; advances to free the prefix
	head     []atomic.Pointer[dequeNode[T]] // per-participant frontier (nil until registered)
	announce []atomic.Pointer[dequeNode[T]] // per-participant current operation
	cursor   atomic.Int64                   // handle allocation cursor
	n        int
}

// NewDeque creates a wait-free deque. maxHandles is the total number of Handle
// registrations allowed over the deque's lifetime (slots are not reclaimable);
// it must be at least 1.
func NewDeque[T any](maxHandles int) *Deque[T] {
	if maxHandles < 1 {
		panic("wf: NewDeque maxHandles must be >= 1")
	}
	sentinel := &dequeNode[T]{}
	sentinel.res.Store(&dequeResult[T]{}) // empty deque
	sentinel.seq.Store(1)
	d := &Deque[T]{
		head:     make([]atomic.Pointer[dequeNode[T]], maxHandles),
		announce: make([]atomic.Pointer[dequeNode[T]], maxHandles),
		n:        maxHandles,
	}
	d.anchor.Store(sentinel)
	return d
}

// DequeHandle is a participant's access point to a Deque. It is bound to one slot
// in the helping arrays and is NOT safe for concurrent use by multiple
// goroutines: acquire one per goroutine.
type DequeHandle[T any] struct {
	d   *Deque[T]
	tid int
}

// Handle registers a new participant and returns its DequeHandle. Each call
// permanently consumes one of the maxHandles slots. It panics if more than
// maxHandles handles are requested.
func (d *Deque[T]) Handle() *DequeHandle[T] {
	id := d.cursor.Add(1) - 1
	if id >= int64(d.n) {
		panic("wf: too many handles (exceeds maxHandles passed to NewDeque)")
	}
	// Register lazily at the current frontier, not the original sentinel, so an
	// idle slot never pins the operation history.
	frontier := d.anchor.Load()
	d.head[id].Store(frontier)
	d.announce[id].Store(frontier)
	return &DequeHandle[T]{d: d, tid: int(id)}
}

// PushFront inserts v at the front (left end) of the deque.
func (h *DequeHandle[T]) PushFront(v T) { h.d.apply(h.tid, dqPushFront, v) }

// PushBack inserts v at the back (right end) of the deque.
func (h *DequeHandle[T]) PushBack(v T) { h.d.apply(h.tid, dqPushBack, v) }

// PopFront removes and returns the value at the front of the deque. The second
// return value is false if the deque was empty during the operation.
func (h *DequeHandle[T]) PopFront() (T, bool) {
	r := h.d.apply(h.tid, dqPopFront, *new(T))
	return r.val, r.ok
}

// PopBack removes and returns the value at the back of the deque. The second
// return value is false if the deque was empty during the operation.
func (h *DequeHandle[T]) PopBack() (T, bool) {
	r := h.d.apply(h.tid, dqPopBack, *new(T))
	return r.val, r.ok
}

// maxFrontier returns the furthest known operation node (largest seq), used as
// the starting point for a new operation.
func (d *Deque[T]) maxFrontier() *dequeNode[T] {
	best := d.anchor.Load()
	bestSeq := best.seq.Load()
	for i := 0; i < d.n; i++ {
		if nd := d.head[i].Load(); nd != nil {
			if sq := nd.seq.Load(); sq > bestSeq {
				best, bestSeq = nd, sq
			}
		}
	}
	return best
}

func (d *Deque[T]) apply(tid int, op dequeOp, arg T) *dequeResult[T] {
	mine := &dequeNode[T]{op: op, arg: arg}
	d.announce[tid].Store(mine)
	d.head[tid].Store(d.maxFrontier())

	for mine.seq.Load() == 0 {
		before := d.head[tid].Load()
		// Round-robin: help the participant whose turn follows this position,
		// otherwise append our own operation.
		var prefer *dequeNode[T]
		helpIdx := (before.seq.Load() + 1) % int64(d.n)
		if cand := d.announce[helpIdx].Load(); cand != nil && cand.seq.Load() == 0 {
			prefer = cand
		} else {
			prefer = mine
		}
		before.next.CompareAndSwap(nil, prefer) // consensus on the successor
		after := before.next.Load()
		if after.seq.Load() == 0 {
			d.publish(before, after)
		}
		d.head[tid].Store(after)
	}

	r := mine.res.Load()
	d.head[tid].Store(mine)
	d.advanceAnchor(mine)
	return r
}

// publish computes after's result from before's result and publishes it, then
// its sequence number. res is set before seq so a reader that sees seq != 0 is
// guaranteed to see res. Both stores are idempotent: every helper computes the
// same values.
func (d *Deque[T]) publish(before, after *dequeNode[T]) {
	br := before.res.Load()
	var nr *dequeResult[T]
	switch after.op {
	case dqPushFront:
		nr = &dequeResult[T]{root: treePushFront(br.root, after.arg)}
	case dqPushBack:
		nr = &dequeResult[T]{root: treePushBack(br.root, after.arg)}
	case dqPopFront:
		root, v, ok := treePopFront(br.root)
		nr = &dequeResult[T]{root: root, val: v, ok: ok}
	default: // dqPopBack
		root, v, ok := treePopBack(br.root)
		nr = &dequeResult[T]{root: root, val: v, ok: ok}
	}
	after.res.CompareAndSwap(nil, nr)
	after.seq.CompareAndSwap(0, before.seq.Load()+1)
}

// advanceAnchor moves the anchor forward to n if n is further along, letting the
// garbage collector reclaim the now-unreachable prefix of the operation list.
func (d *Deque[T]) advanceAnchor(n *dequeNode[T]) {
	for {
		cur := d.anchor.Load()
		if n.seq.Load() <= cur.seq.Load() {
			return
		}
		if d.anchor.CompareAndSwap(cur, n) {
			return
		}
	}
}
