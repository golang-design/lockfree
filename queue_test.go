package lockfree_test

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/changkun/lockfree"
	"github.com/changkun/lockfree/blocking"
)

func TestQueueDequeueEmpty(t *testing.T) {
	q := lockfree.NewQueue()
	if q.Dequeue() != nil {
		t.Fatalf("dequeue empty queue returns non-nil")
	}
}

func TestQueue_Length(t *testing.T) {
	q := lockfree.NewQueue()
	if q.Length() != 0 {
		t.Fatalf("empty queue has non-zero length")
	}

	q.Enqueue(1)
	if q.Length() != 1 {
		t.Fatalf("count of enqueue wrong, want %d, got %d.", 1, q.Length())
	}

	q.Dequeue()
	if q.Length() != 0 {
		t.Fatalf("count of dequeue wrong, want %d, got %d", 0, q.Length())
	}
}

func ExampleQueue() {
	q := lockfree.NewQueue()

	q.Enqueue("1st item")
	q.Enqueue("2nd item")
	q.Enqueue("3rd item")

	fmt.Println(q.Dequeue())
	fmt.Println(q.Dequeue())
	fmt.Println(q.Dequeue())

	// Output:
	// 1st item
	// 2nd item
	// 3rd item
}

type queueInterface interface {
	Enqueue(interface{})
	Dequeue() interface{}
}

type mutexQueue struct {
	q  *blocking.Queue
	mu sync.Mutex
}

func newMutexQueue() *mutexQueue {
	return &mutexQueue{q: blocking.NewQueue()}
}

func (q *mutexQueue) Enqueue(v interface{}) {
	q.mu.Lock()
	q.q.Enqueue(v)
	q.mu.Unlock()
}

func (q *mutexQueue) Dequeue() interface{} {
	q.mu.Lock()
	v := q.q.Dequeue()
	q.mu.Unlock()
	return v
}

func BenchmarkQueue(b *testing.B) {
	length := 1 << 12
	inputs := make([]int, length)
	for i := 0; i < length; i++ {
		inputs = append(inputs, rand.Int())
	}
	q, mq := lockfree.NewQueue(), newMutexQueue()
	b.ResetTimer()

	for _, q := range [...]queueInterface{q, mq} {
		b.Run(fmt.Sprintf("%T", q), func(b *testing.B) {
			var c int64
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := int(atomic.AddInt64(&c, 1)-1) % length
					v := inputs[i]
					if v >= 0 {
						q.Enqueue(v)
					} else {
						q.Dequeue()
					}
				}
			})
		})
	}
}
