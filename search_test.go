// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree_test

import (
	"testing"

	"golang.design/x/lockfree"
)

func TestBinarySearch(t *testing.T) {
	less := func(a, b int) bool { return a < b }
	tests := []struct {
		input []int
		x     int
		want  int
	}{
		{input: []int{1, 2, 3, 4, 5, 6, 7}, x: 6, want: 5},
		{input: []int{1, 2, 3, 4, 5, 6, 7}, x: 2, want: 1},
		{input: []int{1, 2, 3, 4, 5, 6, 7}, x: 8, want: -1}, // not found, above range
		{input: []int{1, 2, 3, 4, 5, 6, 7}, x: 0, want: -1}, // not found, below range
		{input: []int{}, x: 2, want: -1},
	}

	for _, tt := range tests {
		r := lockfree.BinarySearch(tt.input, tt.x, less)
		if r != tt.want {
			t.Fatalf("BinarySearch %v of %v: want %v, got %v", tt.x, tt.input, tt.want, r)
		}
	}
}
