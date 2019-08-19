package blocking_test

import (
	"testing"

	"github.com/changkun/lockfree/blocking"
)

func newSkipList() *blocking.SkipList {
	return blocking.NewSkipList(func(a, b interface{}) bool {
		if a.(int) < b.(int) {
			return true
		}
		return false
	})
}

func TestNewSkipList(t *testing.T) {
	if newSkipList() == nil {
		t.Fatalf("%v: got nil", t.Name())
	}
}

func TestSkipList_Len(t *testing.T) {
	sl := newSkipList()
	if sl == nil {
		t.Fatalf("%v: got nil", t.Name())
	}

	if got := sl.Len(); got != 0 {
		t.Fatalf("Len: got %d, want %d", got, 0)
	}

	for i := 0; i < 10000; i++ {
		sl.Set(i, i)
	}

	if got := sl.Len(); got != 10000 {
		t.Fatalf("Len: got %d, want %d", got, 10000)
	}
}

func TestSkipList_GetFail(t *testing.T) {
	sl := newSkipList()
	if sl == nil {
		t.Fatalf("%v: got nil", t.Name())
	}

	v, ok := sl.Get(-1)
	if ok {
		t.Fatalf("%v: suppose to fail, but got: %v, %v", t.Name(), v, ok)
	}
}

func TestSkipList_GetSuccess(t *testing.T) {
	sl := newSkipList()
	if sl == nil {
		t.Fatalf("%v: got nil", t.Name())
	}

	sl.Set(1, 2)
	if got, ok := sl.Get(1); got != 2 || ok != true {
		t.Fatalf("got %v, %v want %v, %v", got, ok, 2, true)
	}

	sl.Set(1, 3)
	if got, ok := sl.Get(1); got != 3 || ok != true {
		t.Fatalf("got %v, %v want %v, %v", got, ok, 3, true)
	}
}

func TestSkipList_Search(t *testing.T) {
	sl := newSkipList()
	if sl == nil {
		t.Fatalf("%v: got nil", t.Name())
	}

	if ok := sl.Search(1); ok {
		t.Fatalf("got %v want %v", ok, false)
	}

	sl.Set(1, 2)

	if got := sl.Len(); got != 1 {
		t.Fatalf("Len: got %d, want %d", got, 1)
	}

	if ok := sl.Search(1); !ok {
		t.Fatalf("got %v want %v", ok, true)
	}

	if v, ok := sl.Del(1); v != 2 || !ok {
		t.Fatalf("got %v,%v want %d", v, ok, 2)
	}

	if got := sl.Len(); got != 0 {
		t.Fatalf("Len: got %d, want %d", got, 1)
	}
}

func TestSkiplist_Del(t *testing.T) {
	sl := newSkipList()
	if sl == nil {
		t.Fatalf("%v: got nil", t.Name())
	}

	for i := 0; i < 10; i++ {
		sl.Set(i, i)
	}

	for i := 0; i < 100; i++ {
		if _, ok := sl.Del(i); i > 10 && ok {
			t.Fatalf("%v: should fail, got: %v", t.Name(), ok)
		}
	}

	if got := sl.Len(); got != 0 {
		t.Fatalf("Len: got %d, want %d", got, 0)
	}
}

func TestSkipList_Range(t *testing.T) {
	sl := newSkipList()
	if sl == nil {
		t.Fatalf("%v: got nil", t.Name())
	}

	for i := 0; i < 100; i++ {
		sl.Set(i, i)
	}

	current := 10
	sl.Range(10, 20, func(v interface{}) {
		if v != current {
			t.Fatalf("range failed, want %v, got %v", current, v)
		}
		current++
	})

	current = 90
	sl.Range(90, 120, func(v interface{}) {
		if v != current {
			t.Fatalf("range failed, want %v, got %v", current, v)
		}
		current++
	})
	if current != 99 {
		t.Fatalf("range out of bound, want %v, got %v", 99, current)
	}
}
