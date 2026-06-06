// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf

import (
	"math"
	"math/rand/v2"
	"testing"
)

// checkAVL verifies the AVL invariants at every node and returns the subtree
// height and element count: the stored height field is correct, and no node's
// child heights differ by more than one. A violation means the tree is not
// balanced, which would break the O(log n) bound the wait-free deque relies on.
func checkAVL[T any](t *testing.T, n *treeNode[T]) (h, count int) {
	if n == nil {
		return 0, 0
	}
	lh, lc := checkAVL(t, n.left)
	rh, rc := checkAVL(t, n.right)
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

// inorder returns the tree's elements front to back.
func inorder[T any](n *treeNode[T], out *[]T) {
	if n == nil {
		return
	}
	inorder(n.left, out)
	*out = append(*out, n.val)
	inorder(n.right, out)
}

// TestPersistentDequeTree exercises the immutable AVL tree directly against a
// slice reference over a long random sequence of all four operations, checking
// after every step that the in-order sequence matches and the AVL invariant
// holds, and finally that the height never exceeds the AVL bound for its size
// (so the O(log n) per-operation cost is real, not amortized over a degenerate
// shape).
func TestPersistentDequeTree(t *testing.T) {
	const ops = 50000
	rng := rand.New(rand.NewPCG(42, 1))
	var root *treeNode[int]
	var ref []int
	maxHeight, maxCount := 0, 0

	for i := 0; i < ops; i++ {
		switch rng.IntN(4) {
		case 0:
			root = treePushFront(root, i)
			ref = append([]int{i}, ref...)
		case 1:
			root = treePushBack(root, i)
			ref = append(ref, i)
		case 2:
			var v int
			var ok bool
			root, v, ok = treePopFront(root)
			if len(ref) == 0 {
				if ok {
					t.Fatalf("PopFront empty returned (%d, true)", v)
				}
			} else {
				if !ok || v != ref[0] {
					t.Fatalf("PopFront: got (%d,%v), want (%d,true)", v, ok, ref[0])
				}
				ref = ref[1:]
			}
		case 3:
			var v int
			var ok bool
			root, v, ok = treePopBack(root)
			if len(ref) == 0 {
				if ok {
					t.Fatalf("PopBack empty returned (%d, true)", v)
				}
			} else {
				want := ref[len(ref)-1]
				if !ok || v != want {
					t.Fatalf("PopBack: got (%d,%v), want (%d,true)", v, ok, want)
				}
				ref = ref[:len(ref)-1]
			}
		}

		h, count := checkAVL(t, root)
		if count != len(ref) {
			t.Fatalf("tree size %d, want %d", count, len(ref))
		}
		var got []int
		inorder(root, &got)
		for j := range ref {
			if got[j] != ref[j] {
				t.Fatalf("op %d: in-order mismatch at %d: got %d, want %d", i, j, got[j], ref[j])
			}
		}
		if h > maxHeight {
			maxHeight, maxCount = h, count
		}
	}

	// The height of an AVL tree with N nodes is at most ~1.4404*log2(N+2)-0.328.
	if maxCount > 0 {
		bound := 1.4405*math.Log2(float64(maxCount)+2) - 0.328
		if float64(maxHeight) > bound {
			t.Fatalf("height %d exceeds AVL bound %.2f for %d nodes (not balanced)", maxHeight, bound, maxCount)
		}
	}
}
