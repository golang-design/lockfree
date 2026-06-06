// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree

// Less defines a function that compares the order of a and b.
// It returns true if a < b.
type Less[T any] func(a, b T) bool
