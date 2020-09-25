// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree_test

import (
	"reflect"
	"testing"

	"golang.design/x/lockfree"
)

func TestRingBuffer(t *testing.T) {
	rb := lockfree.NewRingBuffer(10)

	for i := 0; i < 20; i++ {
		ok := rb.Put(i)
		if i < 10 && !ok {
			t.Errorf("put failed, %v:%v", i, ok)
		}
		if i > 9 && ok {
			t.Errorf("put failed, %v:%v", i, ok)
		}
	}
	v := rb.LookAll()
	want := []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("not equal: %v", v)
	}

	for i := 0; i < 5; i++ {
		v := rb.Get()
		if v != i {
			t.Errorf("get failed, %v:%v", v, i)
		}
	}

	v = rb.LookAll()
	want = []interface{}{5, 6, 7, 8, 9}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("not equal")
	}
}
