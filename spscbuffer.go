package lockfree

type spscBuffer struct {
	pread  int64
	pwrite int64
	size   int64
	buf    []interface{}
}

func newSpscBuffer(n int64) *spscBuffer {
	return &spscBuffer{buf: make([]interface{}, n)}
}

func (b *spscBuffer) empty() bool {
	return b.buf[b.pread] == nil
}

func (b *spscBuffer) available() bool {
	return b.buf[b.pwrite] == nil
}

func (b *spscBuffer) push(v interface{}) bool {
	if !b.available() {
		return false
	}
	// WMB()
	b.buf[b.pwrite] = v
	if b.pwrite+1 > b.size {
		b.pwrite += (1 - b.size)
	} else {
		b.pwrite++
	}
	return true
}

func (b *spscBuffer) pop() interface{} {
	if b.empty() {
		return nil
	}
	v := b.buf[b.pread]
	b.buf[b.pread] = nil
	if b.pread+1 >= b.size {
		b.pread += (1 - b.size)
	} else {
		b.pread++
	}
	return v
}
