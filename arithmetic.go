// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree

import (
	"math"
	"sync/atomic"
	"unsafe"
)

// AddFloat64 atomically adds delta to the float64 at addr and returns the new
// value. float64 has no native atomic add, so this is the standard lock-free
// read-modify-write: load the current bits, compute the sum, and publish it with
// a compare-and-swap, retrying if another writer intervened. The construction is
// lock-free (some racing writer always makes progress) and rests on the
// universality of compare-and-swap for lock-free read-modify-write (Herlihy,
// "Wait-Free Synchronization", ACM TOPLAS 1991).
func AddFloat64(addr *float64, delta float64) (new float64) {
	var old float64
	for {
		old = math.Float64frombits(atomic.LoadUint64((*uint64)(unsafe.Pointer(addr))))
		if atomic.CompareAndSwapUint64((*uint64)(unsafe.Pointer(addr)),
			math.Float64bits(old), math.Float64bits(old+delta)) {
			break
		}
	}
	return
}
