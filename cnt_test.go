package cnet

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestCnet(t *testing.T) {
	t.Run("tcp", func(t *testing.T) {
		t.Run("reuse-base", func(t *testing.T) {
			if err := testTcpService(":8000", TcpOption{TcpKeepAlive: time.Minute, MultiCore: 4, ReusePort: true}); err != nil {
				t.Error(err)
			}
		})
		t.Run("multiCore-base", func(t *testing.T) {
			if err := testTcpService(":8000", TcpOption{TcpKeepAlive: time.Minute, MultiCore: 4}); err != nil {
				t.Error(err)
			}
		})
	})
	t.Run("udp", func(t *testing.T) {
		t.Run("reuse-base", func(t *testing.T) {
			if err := testUdpService(":8000", UdpOption{ReusePort: true}); err != nil {
				t.Error(err)
			}
		})
		t.Run("multiCore-base", func(t *testing.T) {
			if err := testUdpService(":8000", UdpOption{MultiCore: 4}); err != nil {
				t.Error(err)
			}
		})
	})
}

type serverTcpCallback struct {
	connTotal, connected, close int64
	spanDown, spanUp            int64
}

func (sc *serverTcpCallback) OnConnOpened(c Conn) (out []byte, op Operation) {
	var str = "hello client, welcome to connection\n"
	out = []byte(str)
	atomic.AddInt64(&sc.spanUp, int64(len(out)))
	atomic.AddInt64(&sc.connected, 1)
	atomic.AddInt64(&sc.connTotal, 1)
	fmt.Println("conned - ", sc.connected, "connTotal - ", sc.connTotal, "close - ", sc.close)
	fmt.Println("spanDown - ", sc.spanDown, "spanUp - ", sc.spanUp)
	return
}

// 链接关闭时回调
func (sc *serverTcpCallback) OnConnClosed(c Conn, err error) (op Operation) {
	if err != nil {
		fmt.Println("connection err: ", err)
	}
	atomic.AddInt64(&sc.connected, -1)
	atomic.AddInt64(&sc.connTotal, 1)
	atomic.AddInt64(&sc.close, 1)
	return
}

// 读事件触发
func (sc *serverTcpCallback) ConnHandler(c Conn) (out []byte, op Operation) {
	var rcv = c.Read()
	var l = len(rcv)
	atomic.AddInt64(&sc.spanDown, int64(l))
	fmt.Printf("已收到 %s client 发来长度为: %d 的信息: %s\n", c.RemoteAddr().String(), l, rcv)
	out = []byte("receive ")
	c.ShiftN(l)
	out = append(out, rcv...)
	atomic.AddInt64(&sc.spanUp, int64(len(out)))
	return
}

// 唤醒conn时触发 c.wake
func (sc *serverTcpCallback) OnWakenHandler(c Conn) (out []byte, op Operation) {
	return nil, None
}

func testTcpService(addr string, opt TcpOption) error {
	var (
		call serverTcpCallback
	)

	return TcpService(&call, addr, opt)
}

type serverUdpCallback struct {
	spanDown, spanUp int64
}

func (sc *serverUdpCallback) PackHandler(pack []byte, p Pconn) (out []byte, op Operation) {
	fmt.Printf("receive message :%s of client: %s", pack, p.RemoteAddr())
	atomic.AddInt64(&sc.spanDown, int64(len(pack)))
	out = append(out, []byte("reply: ")...)
	out = append(out, pack...)
	atomic.AddInt64(&sc.spanUp, int64(len(out)))
	return
}

func (sc *serverUdpCallback) SendErr(remoteAddr string, err error) {
	fmt.Printf("send error: %v of client: %s", err, remoteAddr)
}
func testUdpService(addr string, opt UdpOption) error {
	var (
		call serverUdpCallback
	)
	return UdpService(&call, addr, opt)
}
