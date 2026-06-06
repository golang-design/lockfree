// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import "golang.design/x/lockfree"

// HashMap is a lock-free hash table (Michael 2002): a fixed array of buckets,
// each a lock-free ordered List, with keys routed to a bucket by their hash.
//
// Progress guarantee: lock-free. Every operation hashes the key (wait-free) and
// performs exactly one operation on the target bucket's lock-free list, so the
// table inherits the list's lock-freedom. With a good hash and a bounded load
// factor, operations are O(1) expected.
//
// The bucket count is fixed at construction: this implementation does not resize
// (lock-free resizing via split-ordered lists, Shalev & Shavit 2006, is a
// possible future addition). Pick numBuckets for the expected element count. Len
// is an approximate count under concurrency.
//
// A less comparator is required in addition to the hash because each bucket is a
// lock-free ordered list, which needs a deterministic order over the keys that
// collide in a bucket.
type HashMap[K, V any] struct {
	buckets []*List[K, V]
	hash    func(K) uint64
	n       uint64
}

// NewHashMap creates a hash table with numBuckets buckets (>= 1), using hash to
// route keys to buckets and less to order keys within a bucket.
func NewHashMap[K, V any](numBuckets int, hash func(K) uint64, less lockfree.Less[K]) *HashMap[K, V] {
	if numBuckets < 1 {
		panic("lf: NewHashMap numBuckets must be >= 1")
	}
	buckets := make([]*List[K, V], numBuckets)
	for i := range buckets {
		buckets[i] = NewList[K, V](less)
	}
	return &HashMap[K, V]{buckets: buckets, hash: hash, n: uint64(numBuckets)}
}

func (h *HashMap[K, V]) bucket(k K) *List[K, V] {
	return h.buckets[h.hash(k)%h.n]
}

// Set inserts key k with value v, or updates the value if k already exists.
func (h *HashMap[K, V]) Set(k K, v V) { h.bucket(k).Set(k, v) }

// Get returns the value stored for k and whether it was found.
func (h *HashMap[K, V]) Get(k K) (v V, ok bool) { return h.bucket(k).Get(k) }

// Del removes key k, returning its value and true if it was present.
func (h *HashMap[K, V]) Del(k K) (v V, ok bool) { return h.bucket(k).Del(k) }

// Contains reports whether k is present.
func (h *HashMap[K, V]) Contains(k K) bool { return h.bucket(k).Contains(k) }

// Len returns the approximate number of elements across all buckets.
func (h *HashMap[K, V]) Len() int {
	total := 0
	for _, b := range h.buckets {
		total += b.Len()
	}
	return total
}
