// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree_test

import (
	"sync"
	"testing"

	"golang.design/x/lockfree"
)

func TestAddFloat64(t *testing.T) {
	vs := []float64{}
	for i := 1; i <= 10; i++ {
		vs = append(vs, float64(i))
	}

	var sum float64
	wg := sync.WaitGroup{}
	wg.Add(10)
	for _, v := range vs {
		lv := v
		go func() {
			lockfree.AddFloat64(&sum, lv)
			wg.Done()
		}()
	}
	wg.Wait()

	if sum != float64(55) {
		t.Fatalf("AddFloat64 wrong, expected 55, got %v", sum)
	}
}
