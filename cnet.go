package cnet

import (
	"github.com/cuckooemm/cnet/internal"
	"net"
)

type Operation int

const (
	None Operation = iota
	Close
	Shutdown
)

type IEventCallback interface {
	// 链接连接时回调
	OnConnOpened(c Conn) (out []byte, op Operation)
	// 链接关闭时回调
	OnConnClosed(c Conn, err error) (op Operation)
	// 读事件触发
	ConnHandler(c Conn) (out []byte, op Operation)
	// 唤醒conn时触发 c.wake
	OnWakenHandler(c Conn) (out []byte, op Operation)
}
type Conn interface {
	// 上下文返回用户定义的上下文。
	Context() (ctx interface{})

	// 设置用户定义的上下文
	SetContext(ctx interface{})

	// 连接的本地套接字地址
	LocalAddr() (addr net.Addr)

	// 连接的远程对端地址
	RemoteAddr() (addr net.Addr)

	// Read从入站环形缓冲区和事件循环缓冲区中读取所有数据，而不会移动“read”指针，不会淘汰缓冲区数据，直到调用ResetBuffer方法为止 。
	Read() (buf []byte)

	// ResetBuffer重置入站环形缓冲区，入站环形缓冲区中的所有数据已被清除。
	ResetBuffer()

	// ReadN从入站环形缓冲区和事件循环缓冲区读取具有给定长度的字节，不会移动“读取”指针，直到调用ShiftN方法，它才会从缓冲区中逐出数据，
	// 而是从入站环形缓冲区中读取数据。 当可用数据的长度等于给定的“n”时，使用缓冲区和事件循环缓冲区，否则它将不会从入站环形缓冲区读取任何数据。
	// 因此，只有在完全知道基于协议的后续TCP流的长度（例如HTTP请求中的Content-Length属性）时，才应使用此方法，该属性指示应从入站环形缓冲区读取多少数据。
	ReadN(n int) (size int, buf []byte)

	// ShiftN将“read”指针移入给定长度的缓冲区中。
	ShiftN(n int) (size int)

	// BufferLength 返回入站环形缓冲区中可用数据的长度。
	BufferLength() (size int)

	// AsyncWrite异步将数据写入客户端/连接，通常在单个goroutine中而不是事件循环goroutine中调用它。
	AsyncWrite(buf []byte) error

	// 唤醒会为此连接触发一个React事件。
	Wake() error

	// 关闭当前连接
	Close() error
}

func TcpService(callback IEventCallback, addr string, opt TcpOption) error {
	var (
		ln  tcpListener
		err error
	)
	if opt.ReusePort {
		ln.ln, err = internal.ReusePortListen("tcp", addr)
	} else {
		ln.ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return err
	}
	//lr.lsraddr = lr.lr.Addr()
	if err = ln.initFd(); err != nil {
		return err
	}
	defer ln.close()
	return startTcpService(callback, &ln, &opt)
}
