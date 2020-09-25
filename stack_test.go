// Copyright 2020 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

package lockfree_test

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

	"golang.design/x/lockfree"
)

func TestStackPopEmpty(t *testing.T) {
	s := lockfree.NewStack()
	if s.Pop() != nil {
		t.Fatal("pop empty stack returns non-nil")
	}
}

func ExampleStack() {
	s := lockfree.NewStack()

	s.Push(1)
	s.Push(2)
	s.Push(3)

	fmt.Println(s.Pop())
	fmt.Println(s.Pop())
	fmt.Println(s.Pop())

	// Output:
	// 3
	// 2
	// 1
}

type stackInterface interface {
	Push(interface{})
	Pop() interface{}
}

type mutexStack struct {
	v  []interface{}
	mu sync.Mutex
}

func newMutexStack() *mutexStack {
	return &mutexStack{v: make([]interface{}, 0)}
}

func (s *mutexStack) Push(v interface{}) {
	s.mu.Lock()
	s.v = append(s.v, v)
	s.mu.Unlock()
}

func (s *mutexStack) Pop() interface{} {
	s.mu.Lock()
	v := s.v[len(s.v)]
	s.v = s.v[:len(s.v)-1]
	s.mu.Unlock()
	return v
}

func BenchmarkStack(b *testing.B) {
	length := 1 << 12
	inputs := make([]int, length)
	for i := 0; i < length; i++ {
		inputs = append(inputs, rand.Int())
	}
	s, ms := lockfree.NewStack(), newMutexStack()
	b.ResetTimer()
	for _, s := range [...]stackInterface{s, ms} {
		b.Run(fmt.Sprintf("%T", s), func(b *testing.B) {
			var c int64
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := int(atomic.AddInt64(&c, 1)-1) % length
					v := inputs[i]
					if v >= 0 {
						s.Push(v)
					} else {
						s.Pop()
					}
				}
			})
		})
	}
}
