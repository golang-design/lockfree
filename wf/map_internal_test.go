// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf

import (
	"math"
	"math/rand/v2"
	"testing"
)

// checkMapAVL verifies, at every node, that the height field is correct, the AVL
// balance factor is within [-1, 1], and the binary-search-tree order holds, and
// returns the subtree height and element count. A violation means the tree is
// not a balanced BST, which would break correctness or the O(log n) bound.
func checkMapAVL(t *testing.T, n *mapTreeNode[int, int], lo, hi int, hasLo, hasHi bool) (h, count int) {
	if n == nil {
		return 0, 0
	}
	if hasLo && !(lo < n.key) {
		t.Fatalf("BST order violated: key %d not greater than left bound %d", n.key, lo)
	}
	if hasHi && !(n.key < hi) {
		t.Fatalf("BST order violated: key %d not less than right bound %d", n.key, hi)
	}
	lh, lc := checkMapAVL(t, n.left, lo, n.key, hasLo, true)
	rh, rc := checkMapAVL(t, n.right, n.key, hi, true, hasHi)
	want := lh
	if rh > want {
		want = rh
	}
	want++
	if n.height != want {
		t.Fatalf("height field %d, want %d", n.height, want)
	}
	if bf := lh - rh; bf < -1 || bf > 1 {
		t.Fatalf("unbalanced node: balance factor %d", bf)
	}
	return want, lc + rc + 1
}

// TestPersistentMapTree exercises the immutable keyed AVL tree directly against a
// builtin map over a long random sequence of Set/Get/Del, checking after every
// step that lookups agree and the tree is a valid balanced BST, and finally that
// the height never exceeds the AVL bound for its size.
func TestPersistentMapTree(t *testing.T) {
	const ops = 50000
	const keyspace = 512
	rng := rand.New(rand.NewPCG(7, 99))
	less := func(a, b int) bool { return a < b }
	var root *mapTreeNode[int, int]
	ref := map[int]int{}
	maxHeight, maxCount := 0, 0

	for i := 0; i < ops; i++ {
		k := int(rng.IntN(keyspace))
		switch rng.IntN(3) {
		case 0:
			v := int(rng.Int())
			root = mapInsert(root, k, v, less)
			ref[k] = v
		case 1:
			gv, gok := mapLookup(root, k, less)
			wv, wok := ref[k]
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("lookup(%d): got (%d,%v), want (%d,%v)", k, gv, gok, wv, wok)
			}
		case 2:
			var gv int
			var gok bool
			root, gv, gok = mapDelete(root, k, less)
			wv, wok := ref[k]
			delete(ref, k)
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("delete(%d): got (%d,%v), want (%d,%v)", k, gv, gok, wv, wok)
			}
		}

		h, count := checkMapAVL(t, root, 0, 0, false, false)
		if count != len(ref) {
			t.Fatalf("tree size %d, want %d", count, len(ref))
		}
		if h > maxHeight {
			maxHeight, maxCount = h, count
		}
	}

	// AVL height is at most ~1.4404*log2(N+2)-0.328.
	if maxCount > 0 {
		bound := 1.4405*math.Log2(float64(maxCount)+2) - 0.328
		if float64(maxHeight) > bound {
			t.Fatalf("height %d exceeds AVL bound %.2f for %d nodes (not balanced)", maxHeight, bound, maxCount)
		}
	}
}
