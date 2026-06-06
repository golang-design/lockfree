// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"testing"

	"golang.design/x/lockfree/wf"
)

// FuzzStack differentially fuzzes the wait-free stack (single handle, so the
// semantics are sequential LIFO) against a slice reference. The input is decoded
// as a stream of (op, value) pairs: even op = push, odd op = pop, with the pop
// result compared to the reference. Run a single pass with `go test`, or fuzz
// with `go test -fuzz=FuzzStack ./wf`.
func FuzzStack(f *testing.F) {
	f.Add([]byte{0, 1, 0, 2, 1, 0, 0, 3, 1, 0, 1, 0})
	f.Add([]byte{1, 0, 1, 0, 0, 9, 0, 8, 1, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		s := wf.NewStack[byte](1)
		h := s.Handle()
		var ref []byte

		for i := 0; i+1 < len(data); i += 2 {
			if data[i]%2 == 0 { // push
				h.Push(data[i+1])
				ref = append(ref, data[i+1])
			} else { // pop
				gv, gok := h.Pop()
				if len(ref) == 0 {
					if gok {
						t.Fatalf("pop empty: got (%v, true)", gv)
					}
					continue
				}
				wv := ref[len(ref)-1]
				ref = ref[:len(ref)-1]
				if !gok || gv != wv {
					t.Fatalf("pop: got (%v,%v), want (%v,true)", gv, gok, wv)
				}
			}
		}
		for len(ref) > 0 { // drain
			gv, gok := h.Pop()
			wv := ref[len(ref)-1]
			ref = ref[:len(ref)-1]
			if !gok || gv != wv {
				t.Fatalf("drain: got (%v,%v), want (%v,true)", gv, gok, wv)
			}
		}
		if v, ok := h.Pop(); ok {
			t.Fatalf("expected empty after drain, got %v", v)
		}
	})
}
