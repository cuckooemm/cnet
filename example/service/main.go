package main

import (
	"fmt"
	"github.com/cuckooemm/cnet"
	"sync/atomic"
	"time"
)

type serverCallback struct {
	connTotal, connected, close int64
	spanDown, spanUp            int64
}

func (sc *serverCallback) OnConnOpened(c cnet.Conn) (out []byte, op cnet.Operation) {
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
func (sc *serverCallback) OnConnClosed(c cnet.Conn, err error) (op cnet.Operation) {
	if err != nil {
		fmt.Println("connection err: ", err)
	}
	atomic.AddInt64(&sc.connected, -1)
	atomic.AddInt64(&sc.close, 1)
	return
}

// 读事件触发
func (sc *serverCallback) ConnHandler(c cnet.Conn) ([]byte, cnet.Operation) {
	var (
		out []byte
		op  cnet.Operation
	)
	var n, rcv = c.Read()
	atomic.AddInt64(&sc.spanDown, int64(n))
	fmt.Printf("已收到 %s client 发来长度为: %d 的信息: %s\n", c.RemoteAddr(), n, rcv)
	out = []byte("receive ")
	out = append(out, rcv...)
	atomic.AddInt64(&sc.spanUp, int64(len(out)))
	c.ShiftN(n)
	_ = c.Wake()
	return out, op
}

// 唤醒conn时触发 c.wake
func (sc *serverCallback) OnWakenHandler(c cnet.Conn) (out []byte, op cnet.Operation) {
	fmt.Printf("%s 唤醒了\n", c.RemoteAddr())
	return nil, cnet.None
}

// udp 协议需实现
func (sc *serverCallback) PackHandler(pack []byte, p cnet.Pconn) (out []byte, op cnet.Operation) {
	fmt.Printf("receive message :%s of client: %s\n", pack, p.RemoteAddr())
	atomic.AddInt64(&sc.spanDown, int64(len(pack)))
	out = []byte("reply: ")
	out = append(out, pack...)
	atomic.AddInt64(&sc.spanUp, int64(len(out)))
	return
}

// udp 协议需实现
func (sc *serverCallback) SendErr(remoteAddr string, err error) {
	fmt.Printf("send error: %v of client: %s", err, remoteAddr)
}

func main() {
	var (
		c   cnet.Cnet
		err error
	)
	c = cnet.Cnet{
		Network:      cnet.Udp,
		Callback:     &serverCallback{},
		Addr:         ":8000",
		MultiCore:    4,
		TcpKeepAlive: time.Minute,
		ReusePort:    true,
	}
	if err = c.Listener(); err != nil {
		println(err)
	}
}
