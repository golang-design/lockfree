// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"testing"

	"golang.design/x/lockfree/wf"
)

// FuzzDeque differentially fuzzes the wait-free deque (single handle, so the
// semantics are sequential) against a slice reference over all four operations.
// The input is decoded as a stream of (op, value) pairs; op%4 selects
// PushFront/PushBack/PopFront/PopBack, and pop results are compared. Run a single
// pass with `go test`, or fuzz with `go test -fuzz=FuzzDeque ./wf`.
func FuzzDeque(f *testing.F) {
	f.Add([]byte{0, 1, 1, 2, 2, 0, 3, 0, 0, 3, 1, 4, 2, 0, 3, 0})
	f.Add([]byte{2, 0, 3, 0, 0, 9, 1, 8, 2, 0, 3, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		d := wf.NewDeque[byte](1)
		h := d.Handle()
		var ref []byte

		popFront := func() (byte, bool) {
			if len(ref) == 0 {
				return 0, false
			}
			x := ref[0]
			ref = ref[1:]
			return x, true
		}
		popBack := func() (byte, bool) {
			if len(ref) == 0 {
				return 0, false
			}
			x := ref[len(ref)-1]
			ref = ref[:len(ref)-1]
			return x, true
		}

		for i := 0; i+1 < len(data); i += 2 {
			val := data[i+1]
			switch data[i] % 4 {
			case 0:
				h.PushFront(val)
				ref = append([]byte{val}, ref...)
			case 1:
				h.PushBack(val)
				ref = append(ref, val)
			case 2:
				gv, gok := h.PopFront()
				wv, wok := popFront()
				if gok != wok || (wok && gv != wv) {
					t.Fatalf("PopFront: got (%v,%v), want (%v,%v)", gv, gok, wv, wok)
				}
			case 3:
				gv, gok := h.PopBack()
				wv, wok := popBack()
				if gok != wok || (wok && gv != wv) {
					t.Fatalf("PopBack: got (%v,%v), want (%v,%v)", gv, gok, wv, wok)
				}
			}
		}
		for len(ref) > 0 {
			gv, gok := h.PopFront()
			wv, _ := popFront()
			if !gok || gv != wv {
				t.Fatalf("drain: got (%v,%v), want (%v,true)", gv, gok, wv)
			}
		}
	})
}
