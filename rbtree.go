package lockfree

// import (
// 	"sync/atomic"
// 	"unsafe"
// )

// // Comparable interface defines comprable data
// type Comparable interface {
// 	Less(v interface{}) bool
// }

// // RBTree implements a lock-free red black tree
// type RBTree struct {
// 	root unsafe.Pointer // *rbtreeitem
// }

// // NewRBTree ...
// func (rb *RBTree) NewRBTree() *RBTree {
// 	return &RBTree{}
// }

// // Put inserts a value with given comprable key.
// func (rb *RBTree) Put(k Comparable, v interface{}) {
// 	x := &rbtreeitem{color: red, k: k, v: v}
// restart:
// 	var z = load(rb.root)
// 	for !atomic.CompareAndSwapInt32(&rbtitem(z).flag, 0, 1) {
// 	}
// 	var y = load(rb.root)
// 	for y != nil { // find insert point z
// 		z = y
// 		if x.k.Less(rbtitem(y).k) {
// 			y = load(rbtitem(y).left)
// 		} else {
// 			y = load(rbtitem(y).right)
// 		}
// 		if !atomic.CompareAndSwapInt32(&rbtitem(y).flag, 0, 1) {
// 			atomic.StoreInt32(&rbtitem(z).flag, 0) // release held flag
// 			goto restart
// 		}

// 		if y != nil {
// 			atomic.StoreInt32(&rbtitem(z).flag, 0) // release old y's flag
// 		}
// 	}

// 	x.flag = 1
// 	if !rb.setupLocalAreaForInsert(z) {
// 		atomic.StoreInt32(&rbtitem(z).flag, 0) // release held flag
// 		goto restart
// 	}

// 	// place new item x as child of z
// 	x = rbtitem(z)
// 	if z == rb.root {
// 		atomic.StorePointer(&rb.root, unsafe.Pointer(x))
// 		atomic.StorePointer(&rbtitem(rb.root).left, unsafe.Pointer(x))
// 	} else if x.k.Less(&rbtitem(z).k) {
// 		atomic.StorePointer(&rbtitem(z).left, unsafe.Pointer(x))
// 	} else {
// 		atomic.StorePointer(&rbtitem(z).right, unsafe.Pointer(x))
// 	}

// 	rb.putFixup(x)
// }

// func (rb *RBTree) putFixup(x *rbtreeitem) {
// 	for (x.color == red) {
// 		if x
// 	}
// }

// func (rb *RBTree) setupLocalAreaForInsert(z unsafe.Pointer) bool {
// 	// try to get flags for rest of local area
// 	zp := z // take a copy of our parent pointer
// 	if !atomic.CompareAndSwapInt32(&(rbtitem(zp).flag), 0, 1) {
// 		return false
// 	}
// 	if zp != z { // parent has changed - abort
// 		atomic.StoreInt32(&rbtitem(zp).flag, 0)
// 		return false
// 	}
// 	var uncle unsafe.Pointer
// 	if z == load(rbtitem(z).left) { // uncle is the right child
// 		uncle = load(rbtitem(z).right)
// 	} else { // uncle is the left child
// 		uncle = load(rbtitem(z).left)
// 	}

// 	if !atomic.CompareAndSwapInt32(&rbtitem(uncle).flag, 0, 1) {
// 		atomic.StoreInt32(&rbtitem(z).flag, 0)
// 		return false
// 	}
// 	// now try to get the four intention markers above z.
// 	// the second argument is only useful for deletes so we pass z
// 	// which is not an ancestor of z, and will have no effect.
// 	if !rb.getFlagsAndMarkersAbove(z) {
// 		atomic.StoreInt32(&rbtitem(z).flag, 0)
// 		atomic.StoreInt32(&rbtitem(uncle).flag, 0)
// 		return false
// 	}
// 	return true
// }

// func (rb *RBTree) getFlagsAndMarkersAbove(z unsafe.Pointer) bool {

// }

// func (rb *RBTree) getFlagsForMarkers() bool {

// }

// func (rb *RBTree) releaseFlags(success bool, nodesToRelease unsafe.Pointer) {
// 	// release flags identified in nodesToRelease
// 	for _, nd := range nodesToRelease {
// 		if success { // release flag after successfully moving up
// 			if !rb.isIn(nd, moveUpStruct) {
// 				nd.flag = 0
// 			} else { // nd is in the inherited local area
// 				if rb.isGoalNode(nd, moveUpStruct) {
// 					// release unnedded flags in moveUpStruct
// 					// and discard moveUpStruct
// 				}
// 			}
// 		} else { // release flag after failing to move up
// 			if !rb.isIn(nd, moveUpStruct) {
// 				nd.flag = 0
// 			}
// 		}
// 	}
// }

// func (rb *RBTree) spacingRuleIsSatisfied() {

// }

// // Get searches a value by given key, and returns a copy of the value.
// func (rb *RBTree) Get(k Comparable) interface{} {
// 	return nil
// }

// // Has checks if a value exist by given key.
// func (rb *RBTree) Has(k Comparable) bool {
// 	return true
// }

// // Delete deletes a value by given key.
// func (rb *RBTree) Delete(l Comparable) {
// }

// func (rb *RBTree) deleteFixup() {

// }

// type color int32

// const (
// 	red color = iota
// 	black
// )

// type rbtreeitem struct {
// 	flag  int32 // 1 true, 0 false
// 	color color
// 	left  unsafe.Pointer
// 	right unsafe.Pointer

// 	k Comparable
// 	v interface{}
// }

// func rbtitem(p unsafe.Pointer) *rbtreeitem {
// 	return (*rbtreeitem)(p)
// }

// func load(p unsafe.Pointer) unsafe.Pointer {
// 	return atomic.LoadPointer(&p)
// }
