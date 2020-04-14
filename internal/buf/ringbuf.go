package buf

import (
	"errors"
	"unsafe"
)

const (
	bitSize       = 32 << (^uint(0) >> 63)
	maxIntHeadBit = 1 << (bitSize - 2)
)

var ErrIsEmpty = errors.New("ring-buffer is empty")

// TODO 待更改为一个ring缓冲区完成收发
type RingBuffer struct {
	buf     []byte
	size    int
	mask    int
	r       int // next position to read
	w       int // next position to write
	isEmpty bool
}

func NewRingBuf(size int) *RingBuffer {
	size = ceilToPowerOfTwo(size)
	return &RingBuffer{
		buf:     make([]byte, size),
		size:    size,
		mask:    size - 1,
		isEmpty: true,
	}
}

// 读取数据 不改变指针位置
func (r *RingBuffer) LazyRead(len int) (head []byte, tail []byte) {
	if r.isEmpty || len <= 0 {
		return
	}
	// 写的位置正好在读的位置后面
	if r.w > r.r {
		n := r.w - r.r // Length
		if n > len {
			n = len
		}
		head = r.buf[r.r : r.r+n]
		return
	}
	// 获取数据长度
	n := r.size - r.r + r.w // Length
	if n > len {
		n = len
	}

	if r.r+n <= r.size {
		head = r.buf[r.r : r.r+n]
	} else {
		c1 := r.size - r.r
		head = r.buf[r.r:r.size]
		c2 := n - c1
		tail = r.buf[0:c2]
	}
	return
}

// 读取全部数据 不改变指针
func (r *RingBuffer) LazyReadAll() (head []byte, tail []byte) {
	if r.isEmpty {
		return
	}

	if r.w > r.r {
		head = r.buf[r.r:r.w]
		return
	}

	head = r.buf[r.r:r.size]
	if r.w != 0 {
		tail = r.buf[0:r.w]
	}
	return
}

// 更新读取指针
func (r *RingBuffer) Shift(n int) {
	if n <= 0 {
		return
	}
	if n < r.Length() {
		r.r = (r.r + n) & r.mask
		if r.r == r.w {
			r.isEmpty = true
		}
	} else {
		r.Reset()
	}
}

func (r *RingBuffer) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.isEmpty {
		return 0, ErrIsEmpty
	}

	if r.w > r.r {
		n = r.w - r.r
		if n > len(p) {
			n = len(p)
		}
		copy(p, r.buf[r.r:r.r+n])
		r.r = r.r + n
		if r.r == r.w {
			r.isEmpty = true
		}
		return
	}

	n = r.size - r.r + r.w
	if n > len(p) {
		n = len(p)
	}

	if r.r+n <= r.size {
		copy(p, r.buf[r.r:r.r+n])
	} else {
		c1 := r.size - r.r
		copy(p, r.buf[r.r:r.size])
		c2 := n - c1
		copy(p[c1:], r.buf[0:c2])
	}
	r.r = (r.r + n) & r.mask
	if r.r == r.w {
		r.isEmpty = true
	}

	return n, err
}

func (r *RingBuffer) ReadByte() (b byte, err error) {
	if r.isEmpty {
		return 0, ErrIsEmpty
	}
	b = r.buf[r.r]
	r.r++
	if r.r == r.size {
		r.r = 0
	}
	if r.r == r.w {
		r.isEmpty = true
	}
	return b, err
}

// 写入数据，自动扩容
func (r *RingBuffer) Write(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return 0, nil
	}
	free := r.Free()
	if n > free {
		r.malloc(n - free)
	}

	if r.w >= r.r {
		c1 := r.size - r.w
		if c1 >= n {
			copy(r.buf[r.w:], p)
			r.w += n
		} else {
			copy(r.buf[r.w:], p[:c1])
			c2 := n - c1
			copy(r.buf[0:], p[c1:])
			r.w = c2
		}
	} else {
		copy(r.buf[r.w:], p)
		r.w += n
	}
	if r.w == r.size {
		r.w = 0
	}
	r.isEmpty = false
	return n, err
}

func (r *RingBuffer) WriteByte(c byte) error {
	if r.Free() < 1 {
		r.malloc(1)
	}
	r.buf[r.w] = c
	r.w++

	if r.w == r.size {
		r.w = 0
	}
	r.isEmpty = false
	return nil
}

// 可用空间
func (r *RingBuffer) Free() int {
	if r.r == r.w {
		if r.isEmpty {
			return r.size
		}
		return 0
	}
	if r.w < r.r {
		return r.r - r.w
	}
	return r.size - r.w + r.r
}

// 重置数组
func (r *RingBuffer) Reset() {
	r.r = 0
	r.w = 0
	r.isEmpty = true
}
func (r *RingBuffer) Len() int {
	return len(r.buf)
}
func (r *RingBuffer) Cap() int {
	return r.size
}

// 统计有效数据长度
func (r *RingBuffer) Length() int {
	if r.r == r.w {
		if r.isEmpty {
			return 0
		}
		return r.size
	}
	if r.w > r.r {
		return r.w - r.r
	}
	return r.size - r.r + r.w
}

// 扩容
func (r *RingBuffer) malloc(cap int) {
	newCap := ceilToPowerOfTwo(r.size + cap)
	newBuf := make([]byte, newCap)
	oldLen := r.Length()
	_, _ = r.Read(newBuf)
	r.r = 0
	r.w = oldLen
	r.size = newCap
	r.mask = newCap - 1
	r.buf = newBuf
}

// 写如string
func (r *RingBuffer) WriteString(s string) (n int, err error) {
	var (
		x   = (*[2]uintptr)(unsafe.Pointer(&s))
		h   = [3]uintptr{x[0], x[1], x[1]}
		buf = *(*[]byte)(unsafe.Pointer(&h))
	)
	return r.Write(buf)
}

func (r *RingBuffer) IsFull() bool {
	return r.r == r.w && !r.isEmpty
}
func (r *RingBuffer) IsEmpty() bool {
	return r.isEmpty
}

func ceilToPowerOfTwo(n int) int {
	if n&maxIntHeadBit != 0 && n > maxIntHeadBit {
		panic("argument is too large")
	}
	if n <= 2 {
		return 2
	}
	n--
	n = fillBits(n)
	n++
	return n
}
func fillBits(n int) int {
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return n
}
