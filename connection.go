package cnet

import (
	"github.com/cuckooemm/cnet/internal/buf"
	"github.com/cuckooemm/cnet/internal/netpoll"
	"golang.org/x/sys/unix"
	"net"
	"sync"
)

var (
	tcpConnPoll sync.Pool
	udpPackPoll sync.Pool
)

type conn struct {
	fd                    int             // file descriptor
	ctx                   interface{}     // user-defined context
	loop                  *eventTcpLoop   // connected event-loop
	opened                bool            // connection opened event fired
	localAddr, remoteAddr net.Addr        // local addr and remote addr
	inBuf                 *buf.RingBuffer // buffer for data from client
	outBuf                *buf.RingBuffer // buffer for data that is ready to write to client
}

func init() {
	tcpConnPoll.New = func() interface{} {
		return &conn{}
	}
}
func newTCPConn(fd int, el *eventTcpLoop, sa unix.Sockaddr) *conn {
	var conn = tcpConnPoll.Get().(*conn)
	conn.fd = fd
	conn.loop = el
	conn.localAddr = el.srv.ln.lnAddr
	conn.remoteAddr = netpoll.SocketAddrToTCPOrUnixAddr(sa)
	conn.inBuf = buf.GetRingBuf()
	conn.outBuf = buf.GetRingBuf()
	return conn
}

func (c *conn) releaseTCP() {
	c.opened = false
	c.ctx = nil
	c.localAddr = nil
	c.remoteAddr = nil
	buf.PutRingBuf(c.inBuf)
	buf.PutRingBuf(c.outBuf)
	c.inBuf = nil
	c.outBuf = nil
	tcpConnPoll.Put(c)
}

func (c *conn) open(buf []byte) {
	var (
		n   int
		err error
	)
	// 写入socket 出错or满  写入outBuf
	if n, err = unix.Write(c.fd, buf); err != nil {
		_, _ = c.outBuf.Write(buf)
		return
	}
	if n < len(buf) {
		_, _ = c.outBuf.Write(buf[n:])
	}
}

func (c *conn) write(buf []byte) {
	if buf == nil {
		return
	}
	if !c.outBuf.IsEmpty() {
		_, _ = c.outBuf.Write(buf)
		return
	}
	var (
		n   int
		err error
	)
	if n, err = unix.Write(c.fd, buf); err != nil {
		if err == unix.EAGAIN {
			_, _ = c.outBuf.Write(buf)
			// 监听添加可写事件
			if err = c.loop.poller.ModReadWrite(c.fd); err != nil {
				_ = c.loop.loopCloseConn(c, err)
			}
			return
		}
		_ = c.loop.loopCloseConn(c, err)
		return
	}
	if n < len(buf) {
		_, _ = c.outBuf.Write(buf[n:])
		_ = c.loop.poller.ModReadWrite(c.fd)
	}
}

func (c *conn) Read() []byte {
	var (
		out, head, tail []byte
	)
	head, tail = c.inBuf.LazyReadAll()
	out = make([]byte, 0, len(head)+len(tail))
	out = append(out, head...)
	out = append(out, tail...)
	return out
}

func (c *conn) ResetBuffer() {
	c.inBuf.Reset()
	//bytebuffer.Put(c.byteBuffer)
	//c.byteBuffer = nil
}

func (c *conn) ReadN(n int) (size int, out []byte) {
	inBufLen := c.inBuf.Length()
	if n < 1 || inBufLen < n {
		return
	}
	head, tail := c.inBuf.LazyRead(n)
	size = len(head) + len(tail)
	out = make([]byte, 0, size)
	out = append(out, head...)
	out = append(out, tail...)
	return
}

func (c *conn) ShiftN(n int) (size int) {
	inBufLen := c.inBuf.Length()
	if inBufLen < n || n <= 0 {
		c.ResetBuffer()
		size = inBufLen
		return
	}
	size = n
	c.inBuf.Shift(n)
	return
}

func (c *conn) BufferLength() int {
	return c.inBuf.Length()
}

func (c *conn) AsyncWrite(buf []byte) (err error) {
	return c.loop.poller.Trigger(func() error {
		if c.opened {
			c.write(buf)
		}
		return nil
	})
}

func (c *conn) Wake() error {
	return c.loop.poller.Trigger(func() error {
		return c.loop.loopWake(c)
	})
}
func (c *conn) Close() error {
	return c.loop.poller.Trigger(func() error {
		return c.loop.loopCloseConn(c, nil)
	})
}

func (c *conn) Context() interface{}       { return c.ctx }
func (c *conn) SetContext(ctx interface{}) { c.ctx = ctx }
func (c *conn) LocalAddr() net.Addr        { return c.localAddr }
func (c *conn) RemoteAddr() net.Addr       { return c.remoteAddr }
