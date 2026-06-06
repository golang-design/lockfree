// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree

// OrderedMap is a lock-free ordered map: it keeps key/value pairs sorted by key
// and supports concurrent Put, Get, Del and ordered Range.
//
// It replaces the package's former red-black tree, which was sequential despite
// claiming to be non-blocking. A fully lock-free balanced binary search tree is
// research-grade and impractical in Go, so OrderedMap is backed by the
// lock-free SkipList instead, which delivers the same ordered-map API with the
// same O(log n) expected complexity and a genuine lock-free progress guarantee.
//
// Progress guarantee: lock-free for Put/Del, wait-free for Get (inherited from
// SkipList). Len and Range are weakly consistent under concurrency. See SkipList
// for the detailed argument.
type OrderedMap[K, V any] struct {
	sl *SkipList[K, V]
}

// NewOrderedMap returns an empty ordered map keyed by K and ordered by less.
func NewOrderedMap[K, V any](less Less[K]) *OrderedMap[K, V] {
	return &OrderedMap[K, V]{sl: NewSkipList[K, V](less)}
}

// Put stores value v under key k, replacing any existing value.
func (m *OrderedMap[K, V]) Put(k K, v V) { m.sl.Set(k, v) }

// Get returns the value stored under key k and whether it was found.
func (m *OrderedMap[K, V]) Get(k K) (v V, ok bool) { return m.sl.Get(k) }

// Del removes key k, returning its value and true if it was present.
func (m *OrderedMap[K, V]) Del(k K) (v V, ok bool) { return m.sl.Del(k) }

// Len returns the approximate number of entries in the map.
func (m *OrderedMap[K, V]) Len() int { return m.sl.Len() }

// Range calls op for every key/value with from <= key < to, in ascending order.
func (m *OrderedMap[K, V]) Range(from, to K, op func(k K, v V)) { m.sl.Range(from, to, op) }
