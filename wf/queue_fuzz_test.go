// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"testing"

	"golang.design/x/lockfree/wf"
)

// FuzzQueue differentially fuzzes the wait-free queue (single handle, so the
// semantics are sequential FIFO) against a slice reference. The input is decoded
// as a stream of (op, value) pairs: even op = enqueue, odd op = dequeue, with
// the dequeue result compared to the reference. Run a single pass with
// `go test`, or fuzz with `go test -fuzz=FuzzQueue ./wf`.
func FuzzQueue(f *testing.F) {
	f.Add([]byte{0, 1, 0, 2, 1, 0, 0, 3, 1, 0, 1, 0})
	f.Add([]byte{1, 0, 1, 0, 0, 9, 0, 8, 1, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		q := wf.NewQueue[byte](1)
		h := q.Handle()
		var ref []byte

		for i := 0; i+1 < len(data); i += 2 {
			if data[i]%2 == 0 { // enqueue
				h.Enqueue(data[i+1])
				ref = append(ref, data[i+1])
			} else { // dequeue
				gv, gok := h.Dequeue()
				if len(ref) == 0 {
					if gok {
						t.Fatalf("dequeue empty: got (%v,true), want (_,false)", gv)
					}
					continue
				}
				wv := ref[0]
				ref = ref[1:]
				if !gok || gv != wv {
					t.Fatalf("dequeue: got (%v,%v), want (%v,true)", gv, gok, wv)
				}
			}
		}
		// Drain and compare the remainder.
		for len(ref) > 0 {
			gv, gok := h.Dequeue()
			if !gok || gv != ref[0] {
				t.Fatalf("drain: got (%v,%v), want (%v,true)", gv, gok, ref[0])
			}
			ref = ref[1:]
		}
		if v, ok := h.Dequeue(); ok {
			t.Fatalf("expected empty after drain, got %v", v)
		}
	})
}
