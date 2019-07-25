package lockfree

import (
	"sync/atomic"
)

const (
	big int64 = iota
	small
	empty
)

type elem struct {
	key   int64
	child [2]int64 // big or small
	size  int64
	place int64
}

// Sort implements a wait-free algorithm for sorting array A of N elements using P processors.
// The algorithm is divided into three phase:
// 1. tree building: construct a sorted binary tree whose nodes contain the records of A.
//                   we attach two child pointers to each record of A to point to subtrees
//                   of smaller and larger nodes.
//                   Initially, all pointers have the distinct value `nil`.
// 2. tree summation
// 3. element shuffling.
// ref: http://people.csail.mit.edu/shanir/publications/SUZ-sorting.pdf
func Sort(arr []interface{}, ngoroutine int) []interface{} {
	return nil
}

type sorttree struct {
	A []*elem
}

func (t *sorttree) buildWAT(i int64) {
	// A[1] being the first pivot need not be inserted into the tree
	if i == 1 {
		return
	}

	var (
		parent int64 = 1
		side   int64
	)
	for {
		if t.A[parent].key > t.A[i].key {
			side = small
		} else {
			side = big
		}
		// A goroutine g which is inserting record i first compares its key to the key of
		// the root element, setting `side` to the result of comparasion.
		// CAS operation will successed only if the child is empty,
		// a processor can re-read the child's value after the operation to check success.
		// successful installation of i terminates the routine
		if atomic.CompareAndSwapInt64(&t.A[parent].child[side], empty, i) {
			return
		}
		parent = t.A[parent].child[side]
	}
}

// d is the d-th bit of goroutine
func (t *sorttree) treeSum(i, d int64) int64 {
	if i == empty {
		return 0
	}
	if t.A[i].size > 0 {
		return t.A[i].size
	}

	side := d
	sum := t.treeSum(t.A[i].child[side], d+1)
	sum += t.treeSum(t.A[i].child[1-side], d+1)

	t.A[i].size = sum + 1

	return sum + 1
}

func (t *sorttree) findPlace(i, sub, d int64) {
	if i == empty || t.A[i].place > 0 {
		return
	}

	var s int64
	if t.A[i].child[small] != empty {
		s = t.A[t.A[i].child[small]].size
	}

	t.A[i].place = s + sub + 1

	if d == small {
		t.findPlace(t.A[i].child[small], sub, d+1)
		t.findPlace(t.A[i].child[big], sub+s+1, d+1)
	} else {
		t.findPlace(t.A[i].child[big], sub+s+1, d+1)
		t.findPlace(t.A[i].child[small], sub, d+1)
	}
}
