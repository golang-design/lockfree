// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import (
	"math/bits"
	"sync/atomic"

	"golang.design/x/lockfree"
)

// SplitHashMap is a lock-free resizable hash table using split-ordered lists
// (Shalev & Shavit 2006). Unlike HashMap, which has a fixed bucket count, this
// table grows its bucket count as it fills, and resizing moves no data: all
// items live in a single lock-free ordered list sorted by the bit-reversed hash
// (the "split order"), and a bucket is just a lazily-inserted sentinel node that
// points into the right region of that list. Doubling the bucket count only adds
// sentinels.
//
// Progress guarantee: lock-free. Set and Del are helping CAS loops over the
// underlying marked-pointer list. Get is also lock-free, not wait-free: reaching
// a bucket may lazily initialize it, which inserts a sentinel into the list (a
// CAS loop); once the bucket exists Get is a wait-free traversal. Because of this
// lazy initialization even a read-only workload may allocate bucket sentinels on
// first access. Bucket growth is a single best-effort CAS on the size counter.
// Memory reclamation relies on Go's garbage collector.
//
// Like HashMap it needs a hash plus a less comparator: less gives a deterministic
// total order to regular keys that share a split-order key (a hash collision).
// Count is approximate under concurrency. The table grows up to a fixed maximum
// bucket count, beyond which it degrades gracefully to longer chains.
type SplitHashMap[K, V any] struct {
	segments [soSegments]atomic.Pointer[bucketSegment[K, V]]
	head     *soNode[K, V] // sentinel for bucket 0; start of the list
	size     atomic.Uint64 // active bucket count, a power of two
	count    atomic.Uint64 // number of regular elements
	hash     func(K) uint64
	less     lockfree.Less[K]
}

const (
	soSegmentShift = 8
	soSegmentSize  = 1 << soSegmentShift // bucket pointers per segment
	soMaxBuckets   = 1 << 16             // upper bound on bucket count
	soSegments     = soMaxBuckets / soSegmentSize
	soLoadFactor   = 4 // grow when count exceeds loadFactor * size
)

type bucketSegment[K, V any] [soSegmentSize]atomic.Pointer[soNode[K, V]]

// soNode is a split-ordered list node. dummy nodes are bucket sentinels; their
// soKey is even (bit-reversed bucket index) so they sort before the regular nodes
// (odd soKey) that hash into the bucket.
type soNode[K, V any] struct {
	soKey uint64
	key   K
	val   atomic.Pointer[V]
	dummy bool
	next  atomicMarkable[soNode[K, V]]
}

// atomicMarkable is a generic AtomicMarkableReference over node type N. A nil
// stored value reads as (nil, false).
type atomicMarkable[N any] struct {
	p atomic.Pointer[markedVal[N]]
}

type markedVal[N any] struct {
	ref  *N
	mark bool
}

func (m *atomicMarkable[N]) get() (ref *N, mark bool) {
	if c := m.p.Load(); c != nil {
		return c.ref, c.mark
	}
	return nil, false
}

func (m *atomicMarkable[N]) reference() *N {
	if c := m.p.Load(); c != nil {
		return c.ref
	}
	return nil
}

func (m *atomicMarkable[N]) set(ref *N, mark bool) {
	m.p.Store(&markedVal[N]{ref: ref, mark: mark})
}

func (m *atomicMarkable[N]) compareAndSet(expectRef, newRef *N, expectMark, newMark bool) bool {
	c := m.p.Load()
	var cr *N
	var cm bool
	if c != nil {
		cr, cm = c.ref, c.mark
	}
	if cr != expectRef || cm != expectMark {
		return false
	}
	if newRef == cr && newMark == cm {
		return true
	}
	return m.p.CompareAndSwap(c, &markedVal[N]{ref: newRef, mark: newMark})
}

// NewSplitHashMap creates an empty split-ordered hash table using hash to place
// keys and less to break ties between keys that collide on a hash.
func NewSplitHashMap[K, V any](hash func(K) uint64, less lockfree.Less[K]) *SplitHashMap[K, V] {
	h := &SplitHashMap[K, V]{hash: hash, less: less}
	h.head = &soNode[K, V]{soKey: 0, dummy: true} // bucket 0 sentinel
	h.size.Store(2)
	h.bucketPtr(0).Store(h.head)
	return h
}

// soRegular is the split-order key of a regular element with the given hash.
func soRegular(h uint32) uint64 { return uint64(bits.Reverse32(h | 0x80000000)) }

// soDummy is the split-order key of bucket b's sentinel.
func soDummy(b uint64) uint64 { return uint64(bits.Reverse32(uint32(b))) }

// parentBucket clears the most significant set bit of b.
func parentBucket(b uint64) uint64 {
	return b &^ (uint64(1) << (bits.Len64(b) - 1))
}

func (h *SplitHashMap[K, V]) bucketPtr(i uint64) *atomic.Pointer[soNode[K, V]] {
	seg := i >> soSegmentShift
	s := h.segments[seg].Load()
	if s == nil {
		s = &bucketSegment[K, V]{}
		if !h.segments[seg].CompareAndSwap(nil, s) {
			s = h.segments[seg].Load()
		}
	}
	return &s[i&(soSegmentSize-1)]
}

// initBucket ensures bucket b's sentinel exists in the list and returns it,
// recursively initializing the parent bucket first.
func (h *SplitHashMap[K, V]) initBucket(b uint64) *soNode[K, V] {
	ptr := h.bucketPtr(b)
	if n := ptr.Load(); n != nil {
		return n
	}
	start := h.head
	if b != 0 {
		start = h.initBucket(parentBucket(b))
	}
	dummy := &soNode[K, V]{soKey: soDummy(b), dummy: true}
	got, _ := h.listInsert(start, dummy)
	ptr.CompareAndSwap(nil, got)
	return ptr.Load()
}

func (h *SplitHashMap[K, V]) bucket(hash uint32) *soNode[K, V] {
	b := uint64(hash) % h.size.Load()
	n := h.bucketPtr(b).Load()
	if n == nil {
		n = h.initBucket(b)
	}
	return n
}

// nodeLess reports whether node n is ordered strictly before the (soKey, key)
// target. Dummies have even soKeys and regulars odd, so an equal soKey implies
// both are regular and the comparison falls back to less.
func (h *SplitHashMap[K, V]) nodeLess(n *soNode[K, V], soKey uint64, key K) bool {
	if n.soKey != soKey {
		return n.soKey < soKey
	}
	return h.less(n.key, key)
}

func (h *SplitHashMap[K, V]) nodeEqual(n *soNode[K, V], soKey uint64, key K, dummy bool) bool {
	if n == nil || n.soKey != soKey {
		return false
	}
	if dummy || n.dummy {
		return n.dummy == dummy
	}
	return !h.less(n.key, key) && !h.less(key, n.key)
}

// search returns pred and curr around the target position, starting from start
// and physically unlinking marked nodes.
func (h *SplitHashMap[K, V]) search(start, target *soNode[K, V]) (pred, curr *soNode[K, V]) {
retry:
	for {
		pred = start
		curr = pred.next.reference()
		for {
			if curr == nil {
				return pred, nil
			}
			succ, marked := curr.next.get()
			for marked {
				if !pred.next.compareAndSet(curr, succ, false, false) {
					continue retry
				}
				curr = succ
				if curr == nil {
					return pred, nil
				}
				succ, marked = curr.next.get()
			}
			if h.nodeLess(curr, target.soKey, target.key) {
				pred = curr
				curr = succ
			} else {
				return pred, curr
			}
		}
	}
}

// listInsert inserts n into the list starting the search at start. It returns the
// node now occupying n's position (the existing one on a duplicate) and whether n
// was inserted.
func (h *SplitHashMap[K, V]) listInsert(start, n *soNode[K, V]) (*soNode[K, V], bool) {
	for {
		pred, curr := h.search(start, n)
		if h.nodeEqual(curr, n.soKey, n.key, n.dummy) {
			return curr, false
		}
		n.next.set(curr, false)
		if pred.next.compareAndSet(curr, n, false, false) {
			return n, true
		}
	}
}

// Set inserts key k with value v, or updates the value if k already exists.
func (h *SplitHashMap[K, V]) Set(k K, v V) {
	hash := uint32(h.hash(k))
	n := &soNode[K, V]{soKey: soRegular(hash), key: k}
	n.val.Store(&v)
	got, inserted := h.listInsert(h.bucket(hash), n)
	if !inserted {
		got.val.Store(&v) // update existing
		return
	}
	c := h.count.Add(1)
	csize := h.size.Load()
	if c > csize*soLoadFactor && csize < soMaxBuckets {
		h.size.CompareAndSwap(csize, csize*2) // best-effort grow
	}
}

// Get returns the value stored for k and whether it was found.
func (h *SplitHashMap[K, V]) Get(k K) (v V, ok bool) {
	hash := uint32(h.hash(k))
	soKey := soRegular(hash)
	curr := h.bucket(hash)
	for curr != nil {
		succ, marked := curr.next.get()
		if marked {
			curr = succ
			continue
		}
		if curr.soKey > soKey {
			return v, false
		}
		if !curr.dummy && curr.soKey == soKey && !h.less(curr.key, k) && !h.less(k, curr.key) {
			return *curr.val.Load(), true
		}
		curr = succ
	}
	return v, false
}

// Contains reports whether k is present.
func (h *SplitHashMap[K, V]) Contains(k K) bool {
	_, ok := h.Get(k)
	return ok
}

// Del removes key k, returning its value and true if it was present.
func (h *SplitHashMap[K, V]) Del(k K) (v V, ok bool) {
	hash := uint32(h.hash(k))
	target := &soNode[K, V]{soKey: soRegular(hash), key: k}
	start := h.bucket(hash)
	for {
		pred, curr := h.search(start, target)
		if !h.nodeEqual(curr, target.soKey, k, false) {
			return v, false
		}
		succ, marked := curr.next.get()
		if marked {
			continue
		}
		if !curr.next.compareAndSet(succ, succ, false, true) { // logically delete (count once)
			continue
		}
		value := *curr.val.Load()
		pred.next.compareAndSet(curr, succ, false, false) // try to unlink
		h.count.Add(^uint64(0))
		return value, true
	}
}

// Len returns the approximate number of elements.
func (h *SplitHashMap[K, V]) Len() int {
	return int(h.count.Load())
}
