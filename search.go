// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree

// BinarySearch implements the binary search algorithm.
// It returns the index if element x is found in arr, sorted by less.
// Otherwise it returns -1.
func BinarySearch[T any](arr []T, x T, less Less[T]) int {
	l, r := 0, len(arr)-1
	for l <= r {
		mid := (l + r) / 2
		if less(arr[mid], x) {
			l = mid + 1
		} else if less(x, arr[mid]) {
			r = mid - 1
		} else {
			return mid
		}
	}
	return -1
}
