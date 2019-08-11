package lockfree_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/changkun/lockfree"
)

type UniqueRand struct {
	generated map[int]bool
}

func (u *UniqueRand) Intn(n int) int {
	for {
		i := rand.Intn(n)
		if !u.generated[i] {
			u.generated[i] = true
			return i
		}
	}
}

func check(arr []int) {
	for i := range arr {
		for j := range arr {
			if i == j {
				continue
			}
			if arr[i] == arr[j] {
				panic(fmt.Sprintf("found equal: %d,%d, %d,%d", i, j, arr[i], arr[j]))
			}
		}
	}
}

func TestRBTreeWithEqual(t *testing.T) {
	tree := lockfree.NewRBTree(func(a, b interface{}) bool {
		if a.(int) < b.(int) {
			return true
		}
		return false
	})
	if tree.Len() != 0 {
		t.Fatalf("want 0, got %d", tree.Len())
	}
	tree.Del(0)
	if tree.Len() != 0 {
		t.Fatalf("want 0, got %d", tree.Len())
	}

	for i := 0; i <= 5; i++ {
		tree.Put(i, i)
	}
	want := `RBTree
│           ┌── 5
│       ┌── 4
│   ┌── 3
│   │   └── 2
└── 1
    └── 0
`
	if tree.String() != want {
		t.Fatal("unexpected: ", tree.String())
	}

	tree.Put(1, 2)
	if tree.Len() != 6 {
		t.Fatalf("want 6, got %d", tree.Len())
	}
	tree.Del(1)
	if tree.Len() != 5 {
		t.Fatalf("want 5, got %d", tree.Len())
	}
	tree.Del(2)
	if tree.Len() != 4 {
		t.Fatalf("want 4, got %d", tree.Len())
	}
	if tree.Get(10) != nil {
		t.Fatalf("want nil, got %d", tree.Get(10))
	}

}

func TestRBTreeNoEqual(t *testing.T) {
	N := 1000
	for i := 0; i < N; i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			tree := lockfree.NewRBTree(func(a, b interface{}) bool {
				if a.(int) < b.(int) {
					return true
				}
				return false
			})

			// generate unique numbers
			nums := make([]int, i)
			ur := UniqueRand{generated: map[int]bool{}}
			for ii := range nums {
				nums[ii] = ur.Intn(2 * N)
			}
			check(nums)

			// range all numbers and put into tree
			for _, ii := range nums {
				tree.Put(ii, ii)
			}

			// range all numbers and check get is success
			for _, ii := range nums {
				if tree.Get(ii) != ii {
					t.Fatalf("want %v, got %v", ii, tree.Get(ii))
				}
			}

			// check length is correctly equal to len(numbers)
			if tree.Len() != len(nums) {
				t.Fatalf("want %v, got %v", len(nums), tree.Len())
			}

			// range all nums and delete them all
			for _, v := range nums {
				tree.Del(v)
			}

			if tree.Len() != 0 {
				fmt.Println(tree.String())
				t.Fatalf("want %v, got %v", 0, tree.Len())
			}
			// for _, ii := range nums {
			// 	if tree.Get(ii) != nil {
			// 		t.Fatalf("want %v, got %v", nil, tree.Get(ii))
			// 	}
			// }
		})
	}
}

func BenchmarkRBTree(b *testing.B) {
	for size := 0; size < 1000; size += 10 {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			tree := lockfree.NewRBTree(func(a, b interface{}) bool {
				if a.(int) < b.(int) {
					return true
				}
				return false
			})
			for i := 0; i < b.N; i++ {
				for n := 0; n < size; n++ {
					tree.Put(n, n)
				}
			}
		})
	}
}
