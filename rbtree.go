package lockfree

import "fmt"

type color uint32

const (
	red color = iota
	black
)

type rbnode struct {
	c      color
	left   *rbnode
	right  *rbnode
	parent *rbnode
	k, v   interface{}
}

func (n *rbnode) color() color {
	if n == nil {
		return black
	}
	return n.c
}

func (n *rbnode) grandparent() *rbnode {
	return n.parent.parent
}

func (n *rbnode) uncle() *rbnode {
	if n.parent == n.grandparent().left {
		return n.grandparent().right
	}
	return n.grandparent().left
}

func (n *rbnode) sibling() *rbnode {
	if n == n.parent.left {
		return n.parent.right
	}
	return n.parent.left
}

func (n *rbnode) maximumNode() *rbnode {
	for n.right != nil {
		n = n.right
	}
	return n
}

// RBTree is a red-black tree
type RBTree struct {
	root *rbnode
	len  int
	less Less
}

// Less returns true if a < b
type Less func(a, b interface{}) bool

// NewRBTree creates a red-black tree
func NewRBTree(less Less) *RBTree {
	return &RBTree{less: less}
}

// Len returns the size of the tree
func (t *RBTree) Len() int {
	return t.len
}

// Put stores the value by given key
func (t *RBTree) Put(key, value interface{}) {
	var insertedNode *rbnode

	new := &rbnode{k: key, v: value, c: red}
	if t.root != nil {
		node := t.root
	LOOP:
		for {
			switch {
			case t.less(key, node.k):
				if node.left == nil {
					node.left = new
					insertedNode = node.left
					break LOOP
				}
				node = node.left
			case t.less(node.k, key):
				if node.right == nil {
					node.right = new
					insertedNode = node.right
					break LOOP
				}
				node = node.right
			default: // =
				node.k = key
				node.v = value
				return
			}
		}
		insertedNode.parent = node
	} else {
		t.root = new
		insertedNode = t.root
	}
	t.insertCase1(insertedNode)
	t.len++
}

func (t *RBTree) insertCase1(n *rbnode) {
	if n.parent == nil {
		n.c = black
		return
	}
	t.insertCase2(n)
}
func (t *RBTree) insertCase2(n *rbnode) {
	if n.parent.color() == black {
		return
	}
	t.insertCase3(n)
}
func (t *RBTree) insertCase3(n *rbnode) {
	if n.uncle().color() == red {
		n.parent.c = black
		n.uncle().c = black
		n.grandparent().c = red
		t.insertCase1(n.grandparent())
		return
	}
	t.insertCase4(n)

}
func (t *RBTree) insertCase4(n *rbnode) {
	if n == n.parent.right && n.parent == n.grandparent().left {
		t.rotateLeft(n.parent)
		n = n.left
	} else if n == n.parent.left && n.parent == n.grandparent().right {
		t.rotateRight(n.parent)
		n = n.right
	}
	t.insertCase5(n)
}
func (t *RBTree) insertCase5(n *rbnode) {
	n.parent.c = black
	n.grandparent().c = red
	if n == n.parent.left && n.parent == n.grandparent().left {
		t.rotateRight(n.grandparent())
		return
	} else if n == n.parent.right && n.parent == n.grandparent().right {
		t.rotateLeft(n.grandparent())
	}
}

func (t *RBTree) replace(old, new *rbnode) {
	if old.parent == nil {
		t.root = new
	} else {
		if old == old.parent.left {
			old.parent.left = new
		} else {
			old.parent.right = new
		}
	}
	if new != nil {
		new.parent = old.parent
	}
}

func (t *RBTree) rotateLeft(n *rbnode) {
	right := n.right
	t.replace(n, right)
	n.right = right.left
	if right.left != nil {
		right.left.parent = n
	}
	right.left = n
	n.parent = right
}
func (t *RBTree) rotateRight(n *rbnode) {
	left := n.left
	t.replace(n, left)
	n.left = left.right
	if left.right != nil {
		left.right.parent = n
	}
	left.right = n
	n.parent = left
}

// Get returns the stored value by given key
func (t *RBTree) Get(key interface{}) interface{} {
	n := t.find(key)
	if n == nil {
		return nil
	}
	return n.v
}

func (t *RBTree) find(key interface{}) *rbnode {
	n := t.root
	for n != nil {
		switch {
		case t.less(key, n.k):
			n = n.left
		case t.less(n.k, key):
			n = n.right
		default:
			return n
		}
	}
	return nil
}

// Del deletes the stored value by given key
func (t *RBTree) Del(key interface{}) {
	var child *rbnode

	n := t.find(key)
	if n == nil {
		return
	}

	if n.left != nil && n.right != nil {
		pred := n.left.maximumNode()
		n.k = pred.k
		n.v = pred.v
		n = pred
	}

	if n.left == nil || n.right == nil {
		if n.right == nil {
			child = n.left
		} else {
			child = n.right
		}
		if n.c == black {
			n.c = child.color()
			t.delCase1(n)
		}

		t.replace(n, child)
		if n.parent == nil && child != nil {
			child.c = black
		}
	}
	t.len--
}

func (t *RBTree) delCase1(n *rbnode) {
	if n.parent == nil {
		return
	}

	t.delCase2(n)
}
func (t *RBTree) delCase2(n *rbnode) {
	sibling := n.sibling()
	if sibling.color() == red {
		n.parent.c = red
		sibling.c = black
		if n == n.parent.left {
			t.rotateLeft(n.parent)
		} else {
			t.rotateRight(n.parent)
		}
	}
	t.delCase3(n)
}
func (t *RBTree) delCase3(n *rbnode) {
	sibling := n.sibling()
	if n.parent.color() == black &&
		sibling.color() == black &&
		sibling.left.color() == black &&
		sibling.right.color() == black {
		sibling.c = red
		t.delCase1(n.parent)
		return
	}
	t.delCase4(n)
}
func (t *RBTree) delCase4(n *rbnode) {
	sibling := n.sibling()
	if n.parent.color() == red &&
		sibling.color() == black &&
		sibling.left.color() == black &&
		sibling.right.color() == black {
		sibling.c = red
		n.parent.c = black
		return
	}
	t.delCase5(n)
}
func (t *RBTree) delCase5(n *rbnode) {
	sibling := n.sibling()
	if n == n.parent.left &&
		sibling.color() == black &&
		sibling.left.color() == red &&
		sibling.right.color() == black {
		sibling.c = red
		sibling.left.c = black
		t.rotateRight(sibling)
	} else if n == n.parent.right &&
		sibling.color() == black &&
		sibling.right.color() == red &&
		sibling.left.color() == black {
		sibling.c = red
		sibling.right.c = black
		t.rotateLeft(sibling)
	}
	t.delCase6(n)
}
func (t *RBTree) delCase6(n *rbnode) {
	sibling := n.sibling()
	sibling.c = n.parent.color()
	n.parent.c = black
	if n == n.parent.left && sibling.right.color() == red {
		sibling.right.c = black
		t.rotateLeft(n.parent)
		return
	}
	sibling.left.c = black
	t.rotateRight(n.parent)
}

func (t *RBTree) String() string {
	str := "RBTree\n"
	if t.Len() != 0 {
		t.root.output("", true, &str)
	}
	return str
}

func (n *rbnode) String() string {
	return fmt.Sprintf("%v", n.k)
}

func (n *rbnode) output(prefix string, isTail bool, str *string) {
	if n.right != nil {
		newPrefix := prefix
		if isTail {
			newPrefix += "│   "
		} else {
			newPrefix += "    "
		}
		n.right.output(newPrefix, false, str)
	}
	*str += prefix
	if isTail {
		*str += "└── "
	} else {
		*str += "┌── "
	}
	*str += n.String() + "\n"
	if n.left != nil {
		newPrefix := prefix
		if isTail {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
		n.left.output(newPrefix, true, str)
	}
}
