// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import "testing"

// TestSplitHashMapGrows is a white-box test that the table actually resizes: the
// defining property of split-ordered hashing. Behavioral conformance alone would
// pass even if the bucket count never grew (it would just degrade to one long
// list), so this asserts the internal size counter increased and that the table
// stays correct across the growth.
func TestSplitHashMapGrows(t *testing.T) {
	h := NewSplitHashMap[int, int](func(k int) uint64 { return uint64(k) * 0x9e3779b97f4a7c15 },
		func(a, b int) bool { return a < b })

	start := h.size.Load()
	const n = 20000
	for i := 0; i < n; i++ {
		h.Set(i, i*2)
	}

	if grown := h.size.Load(); grown <= start {
		t.Fatalf("bucket count did not grow: start %d, after %d inserts %d", start, n, grown)
	} else {
		t.Logf("bucket count grew from %d to %d for %d elements", start, grown, n)
	}
	if h.Len() != n {
		t.Fatalf("Len: got %d, want %d", h.Len(), n)
	}
	for i := 0; i < n; i++ {
		if v, ok := h.Get(i); !ok || v != i*2 {
			t.Fatalf("Get(%d): got (%v,%v), want (%d,true)", i, v, ok, i*2)
		}
	}
}
