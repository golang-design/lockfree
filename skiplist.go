package lockfree

import "math/rand"

// A SkipList maintains an ordered collection of key:valkue pairs.
// It support insertion, lookup, and deletion operations with O(log n) time complexity
// Paper: Pugh, William (June 1990). "Skip lists: a probabilistic alternative to balanced
// trees". Communications of the ACM 33 (6): 668–676
type SkipList struct {
	header   *skiplistitem
	len      int
	MaxLevel int
}

// NewSkipList returns a skiplist.
func NewSkipList() *SkipList {
	return &SkipList{
		header:   &skiplistitem{forward: []*skiplistitem{nil}},
		MaxLevel: 32,
	}
}

// Len returns the length of given skiplist.
func (s *SkipList) Len() int {
	return s.len
}

// Set sets given k and v pair into the skiplist.
func (s *SkipList) Set(k Comparator, v interface{}) {
	// s.level starts from 0, we need to allocate one
	update := make([]*skiplistitem, s.level()+1, s.effectiveMaxLevel()+1) // make(type, len, cap)

	x := s.path(s.header, update, k)
	if x != nil && x.k == k { // if key exist, update
		x.v = v
		return
	}

	newl := s.randomLevel()
	if curl := s.level(); newl > curl {
		for i := curl + 1; i <= newl; i++ {
			update = append(update, s.header)
			s.header.forward = append(s.header.forward, nil)
		}
	}

	item := &skiplistitem{
		forward: make([]*skiplistitem, newl+1, s.effectiveMaxLevel()+1),
		k:       k,
		v:       v,
	}
	for i := 0; i <= newl; i++ {
		item.forward[i] = update[i].forward[i]
		update[i].forward[i] = item
	}

	s.len++
}

func (s *SkipList) path(x *skiplistitem, update []*skiplistitem, k Comparator) (candidate *skiplistitem) {
	depth := len(x.forward) - 1
	for i := depth; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].k.Less(k) {
			x = x.forward[i]
		}
		if update != nil {
			update[i] = x
		}
	}
	return x.next()
}

func (s *SkipList) randomLevel() (n int) {
	for n = 0; n < s.effectiveMaxLevel() && rand.Float64() < 0.25; n++ {
	}
	return
}

// Get returns corresponding v with given k.
func (s *SkipList) Get(k Comparator) (v interface{}, ok bool) {
	x := s.path(s.header, nil, k)
	if x == nil || x.k != k {
		return nil, false
	}
	return x.v, true
}

// Search returns true if k is founded in the skiplist.
func (s *SkipList) Search(k Comparator) (ok bool) {
	x := s.path(s.header, nil, k)
	if x != nil {
		ok = true
		return
	}
	return
}

// Range interates `from` to `to` with `op`.
func (s *SkipList) Range(from, to Comparator, op func(v interface{})) {
	for start := s.path(s.header, nil, from); start.next() != nil; start = start.next() {
		if !start.k.Less(to) {
			return
		}

		op(start.v)
	}
}

// Del returns the deleted value if ok
func (s *SkipList) Del(k Comparator) (v interface{}, ok bool) {
	update := make([]*skiplistitem, s.level()+1, s.effectiveMaxLevel())

	x := s.path(s.header, update, k)
	if x == nil || x.k != k {
		ok = false
		return
	}

	v = x.v
	for i := 0; i <= s.level() && update[i].forward[i] == x; i++ {
		update[i].forward[i] = x.forward[i]
	}
	for s.level() > 0 && s.header.forward[s.level()] == nil {
		s.header.forward = s.header.forward[:s.level()]
	}
	s.len--
	ok = true
	return
}

func (s *SkipList) level() int {
	return len(s.header.forward) - 1
}

func (s *SkipList) effectiveMaxLevel() int {
	if s.level() < s.MaxLevel {
		return s.MaxLevel
	}
	return s.level()
}

type skiplistitem struct {
	forward []*skiplistitem
	k       Comparator
	v       interface{}
}

func (s *skiplistitem) next() *skiplistitem {
	if len(s.forward) == 0 {
		return nil
	}
	return s.forward[0]
}

// Comparator defines comparator interface
type Comparator interface {
	Less(v interface{}) bool
}
