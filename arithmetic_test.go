package lockfree_test

import (
	"sync"
	"testing"

	"github.com/changkun/lockfree"
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
