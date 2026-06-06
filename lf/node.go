// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lf

import "sync/atomic"

// directItem is a singly-linked node shared by the lock-free Stack and Queue.
// Its next pointer is accessed atomically so concurrent operations can link
// and unlink nodes with compare-and-swap.
type directItem[T any] struct {
	next atomic.Pointer[directItem[T]]
	v    T
}
