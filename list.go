package lockfree

import (
	"sync/atomic"
	"unsafe"
)

// Element is an element of linked list.
type Element struct {
	prev, next *Element
	list       *List
	Value      interface{}
}

// Next returns the next list element or nil.
func (e *Element) Next() *Element {
	return nil // TODO:
}

// Prev returns the previous list element or nil.
func (e *Element) Prev() *Element {
	return nil // TODO:
}

// List represents a *non-blocking* doubly linked list.
// The zero value for List is an empty list ready to use.
type List struct {
	root Element // sentinel list element, only &root, root.prev, and root.next are used
	len  uint64  // current list length excluding (this) sentinel element
}

// Init initializes or clears list l.
func (l *List) Init() *List {
	p := unsafe.Pointer(l.root.next)
	q := unsafe.Pointer(l.root.prev)
	atomic.StorePointer(&p, unsafe.Pointer(&l.root))
	atomic.StorePointer(&q, unsafe.Pointer(&l.root))
	return l
}

// NewList returns an initialized list.
func NewList() *List {
	return new(List).Init()
}

// Len returns the number of elements of list l.
// The complexity is O(1).
func (l *List) Len() int {
	return int(atomic.LoadUint64(&l.len))
}

// Front returns the first element of list l or nil if the list is empty.
func (l *List) Front() *Element {
	head := unsafe.Pointer(l.root.next)
	e := (*Element)(atomic.LoadPointer(&head))
	if e == &l.root {
		return nil
	}
	return e
}

// Back returns the last element of list l or nil if the list is empty.
func (l *List) Back() *Element {
	back := unsafe.Pointer(l.root.prev)
	e := (*Element)(atomic.LoadPointer(&back))
	if e == &l.root {
		return nil
	}
	return e
}

func (l *List) remove(e *Element) {
	// TODO:
}

// Remove removes e from l if e is an element of list l.
// It returns the element value e.Value.
// The element must not be nil.
func (l *List) Remove(e *Element) interface{} {
	ll := unsafe.Pointer(e.list)
	if (*List)(atomic.LoadPointer(&ll)) == l {
		l.remove(e)
	}
	return e.Value
}

// PushFront inserts a new element e with value v at the front of list l and returns e.
func (l *List) PushFront(v interface{}) *Element {
	return nil // TODO:
}

// PushBack inserts a new element e with value v at the back of list l and return e.
func (l *List) PushBack(v interface{}) *Element {
	return nil // TODO:
}

// InsertBefore inserts a new element e with value v immediately before mark and returns e.
// If mark is not an element
func (l *List) InsertBefore(v interface{}, mark *Element) *Element {
	return nil // TODO:
}

// InsertAfter inserts v after at, increments l.len, and returns e.
func (l *List) InsertAfter(v interface{}, at *Element) *Element {
	// TODO:
	return nil
}

// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
// The elemtn must not be nil.
func (l *List) MoveToFront(e *Element) {
	// TODO:
}

// MoveToBack moves element e to the back of list l.
// If e is not an element of l, the list is not mofified.
// The element must not be nil.
func (l *List) MoveToBack(e *Element) {
	// TODO:
}

// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
func (l *List) MoveBefore(e, mark *Element) {
	// TODO:
}

// MoveAfter moves element e to its new position after mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
func (l *List) MoveAfter(e, mark *Element) {
	// TODO:
}

// PushBackList inserts a copy of an other list at the back of list l.
// The lists l and other may be the same. They must not be nil.
func (l *List) PushBackList(other *List) {
	// TODO:
}

// PushFrontList inserts a copy of an other list at the front of list l.
// The lists l and other may be the same. They must not be nil.
func (l *List) PushFrontList(other *List) {
	// TODO:
}
