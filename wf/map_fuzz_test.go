// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package wf_test

import (
	"testing"

	"golang.design/x/lockfree/wf"
)

// FuzzMap differentially fuzzes the wait-free ordered map (single handle, so the
// semantics are sequential) against a builtin map. The input is decoded as a
// stream of (op, key) triples (op, key, value): op%3 selects Set/Get/Del, and
// Get/Del results are compared. Run a single pass with `go test`, or fuzz with
// `go test -fuzz=FuzzMap ./wf`.
func FuzzMap(f *testing.F) {
	f.Add([]byte{0, 1, 7, 1, 1, 0, 0, 2, 9, 2, 2, 0, 0, 1, 5})
	f.Add([]byte{1, 3, 0, 2, 3, 0, 0, 3, 8, 1, 3, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		m := wf.NewMap[byte, byte](1, func(a, b byte) bool { return a < b })
		h := m.Handle()
		ref := map[byte]byte{}

		for i := 0; i+2 < len(data); i += 3 {
			k, v := data[i+1], data[i+2]
			switch data[i] % 3 {
			case 0:
				h.Set(k, v)
				ref[k] = v
			case 1:
				gv, gok := h.Get(k)
				wv, wok := ref[k]
				if gok != wok || (wok && gv != wv) {
					t.Fatalf("Get(%d): got (%v,%v), want (%v,%v)", k, gv, gok, wv, wok)
				}
			case 2:
				gv, gok := h.Del(k)
				wv, wok := ref[k]
				delete(ref, k)
				if gok != wok || (wok && gv != wv) {
					t.Fatalf("Del(%d): got (%v,%v), want (%v,%v)", k, gv, gok, wv, wok)
				}
			}
		}
		for k := 0; k < 256; k++ { // full sweep: every key must agree
			gv, gok := h.Get(byte(k))
			wv, wok := ref[byte(k)]
			if gok != wok || (wok && gv != wv) {
				t.Fatalf("final Get(%d): got (%v,%v), want (%v,%v)", k, gv, gok, wv, wok)
			}
		}
	})
}
