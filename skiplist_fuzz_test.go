// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree_test

import (
	"testing"

	"golang.design/x/lockfree"
)

// FuzzSkipList differentially fuzzes the lock-free skip list against a plain map
// reference. The fuzz input is decoded as a stream of (op, key, value) triples;
// after replaying them on both, every Get/Del result and the final length must
// agree. Run a single pass with `go test`, or actually fuzz with
// `go test -fuzz=FuzzSkipList`.
func FuzzSkipList(f *testing.F) {
	// Seed corpus: set 5=7, get 5, del 5, get 5.
	f.Add([]byte{0, 5, 7, 2, 5, 0, 1, 5, 0, 2, 5, 0})
	f.Add([]byte{0, 1, 1, 0, 2, 2, 0, 3, 3, 1, 2, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		sl := lockfree.NewSkipList[int, int](func(a, b int) bool { return a < b })
		ref := map[int]int{}

		for i := 0; i+2 < len(data); i += 3 {
			key := int(data[i+1])
			val := int(data[i+2])
			switch data[i] % 3 {
			case 0: // Set
				sl.Set(key, val)
				ref[key] = val
			case 1: // Del
				wv, wok := ref[key]
				delete(ref, key)
				gv, gok := sl.Del(key)
				if gok != wok || (wok && gv != wv) {
					t.Fatalf("Del(%d): got (%v,%v) want (%v,%v)", key, gv, gok, wv, wok)
				}
			case 2: // Get
				wv, wok := ref[key]
				gv, gok := sl.Get(key)
				if gok != wok || (wok && gv != wv) {
					t.Fatalf("Get(%d): got (%v,%v) want (%v,%v)", key, gv, gok, wv, wok)
				}
			}
		}
		if got := sl.Len(); got != len(ref) {
			t.Fatalf("Len: got %d, want %d", got, len(ref))
		}
	})
}
