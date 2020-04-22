package cnet

import (
	"github.com/cuckooemm/cnet/internal/buf"
	"github.com/cuckooemm/cnet/internal/netpoll"
	"golang.org/x/sys/unix"
	"sync"
)

var (
	udpPackPoll = sync.Pool{
		New: func() interface{} {
			return &pack{}
		},
	}
)

type conn struct {
	fd                             int                    // file descriptor
	opened                         bool                   // connection opened event fired
	data                           map[string]interface{} // user-defined context
	loop                           *eventTcpLoop          // connected event-loop
	inBuf, outBuf                  *buf.RingBuffer        // buffer for data from client
	network, localAddr, remoteAddr string                 // network、local addr and remote addr
}

func newTCPConn(fd int, el *eventTcpLoop, sa unix.Sockaddr) *conn {
	var conn = &conn{}
	conn.fd = fd
	conn.data = make(map[string]interface{})
	conn.loop = el
	conn.localAddr = el.srv.localAddr
	conn.remoteAddr = netpoll.SocketAddrToTCPOrUnixAddr(sa).String()
	conn.inBuf = buf.GetRingBuf()
	conn.outBuf = buf.GetRingBuf()
	return conn
}

func (c *conn) releaseTCP() {
	c.opened = false
	c.inBuf.Reset()
	c.outBuf.Reset()
	buf.PutRingBuf(c.inBuf)
	buf.PutRingBuf(c.outBuf)
	c.inBuf = nil
	c.outBuf = nil
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

func (c *conn) Read() (size int, head, tail []byte) {
	head, tail = c.inBuf.LazyReadAll()
	size = len(head) + len(tail)
	return
}

func (c *conn) ResetBuffer() {
	c.inBuf.Reset()
	//bytebuffer.Put(c.byteBuffer)
	//c.byteBuffer = nil
}

func (c *conn) ReadN(n int) (size int, head, tail []byte) {
	inBufLen := c.inBuf.Length()
	if n < 1 || inBufLen < n {
		return
	}
	head, tail = c.inBuf.LazyRead(n)
	size = len(head) + len(tail)
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

func (c *conn) Expand() map[string]interface{}        { return c.data }
func (c *conn) SetExpand(data map[string]interface{}) { c.data = data }
func (c *conn) Network() string                       { return c.network }
func (c *conn) LocalAddr() string                     { return c.localAddr }
func (c *conn) RemoteAddr() string                    { return c.remoteAddr }

type pack struct {
	fd                             int // file descriptor
	sa                             unix.Sockaddr
	network, localAddr, remoteAddr string // network、local and remote addr
}

func newUDPPack(fd int, el *eventUdpLoop, sa unix.Sockaddr) *pack {
	var pack = udpPackPoll.Get().(*pack)
	pack.fd = fd
	pack.sa = sa
	pack.localAddr = el.srv.localAddr
	pack.remoteAddr = netpoll.SocketAddrToUDPAddr(sa).String()
	return pack
}

func (p *pack) releaseUDP() {
	p.sa = nil
	p.localAddr = ""
	p.remoteAddr = ""
	udpPackPoll.Put(p)
}

func (p *pack) SendTo(buf []byte) error {
	return unix.Sendto(p.fd, buf, 0, p.sa)
}

func (p *pack) sendTo(buf []byte) error {
	return unix.Sendto(p.fd, buf, 0, p.sa)
}

func (p *pack) LocalAddr() string  { return p.localAddr }
func (p *pack) RemoteAddr() string { return p.remoteAddr }
func (p *pack) Network() string    { return p.network }
