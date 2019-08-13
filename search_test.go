package lockfree_test

import (
	"testing"

	"github.com/changkun/lockfree"
)

func TestBinarySearch(t *testing.T) {
	tests := []struct {
		input []interface{}
		x     interface{}
		less  lockfree.Less
		want  int
	}{
		{
			input: []interface{}{1, 2, 3, 4, 5, 6, 7},
			x:     6,
			less: func(a, b interface{}) bool {
				if a.(int) < b.(int) {
					return true
				}
				return false
			},
			want: 5,
		},
		{
			input: []interface{}{1, 2, 3, 4, 5, 6, 7},
			x:     2,
			less: func(a, b interface{}) bool {
				if a.(int) < b.(int) {
					return true
				}
				return false
			},
			want: 1,
		},
		{
			input: []interface{}{},
			x:     2,
			less: func(a, b interface{}) bool {
				if a.(int) < b.(int) {
					return true
				}
				return false
			},
			want: -1,
		},
	}

	for _, tt := range tests {
		r := lockfree.BinarySearch(tt.input, tt.x, tt.less)
		if r != tt.want {
			t.Fatalf("BinarySearch %v of %v: want %v, got %v", tt.x, tt.input, tt.want, r)
		}
	}
}
