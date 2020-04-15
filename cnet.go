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

type ITCPEventCallback interface {
	// 链接连接时回调
	OnConnOpened(c Conn) (out []byte, op Operation)
	// 链接关闭时回调
	OnConnClosed(c Conn, err error) (op Operation)
	// 读事件触发
	ConnHandler(c Conn) (out []byte, op Operation)
	// 唤醒conn时触发 c.wake
	OnWakenHandler(c Conn) (out []byte, op Operation)
}
type IUDPEventCallback interface {
	// 读事件触发
	PackHandler(pack []byte, p Pconn) (out []byte, op Operation)
	// 发送数据错误时触发
	SendErr(remoteAddr string, err error)
}
type Conn interface {
	// 返回用户定义的上下文。
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

	// ReadN从入站环形缓冲区和读取具有给定长度的字节，不会移动“读取”指针，直到调用ShiftN方法，它才会从缓冲区中逐出数据，
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

type Pconn interface {
	// 连接的本地套接字地址
	LocalAddr() (addr string)

	// 连接的远程对端地址
	RemoteAddr() (addr string)

	// SendTo为UDP套接字写入数据，它允许您在各个goroutine中将数据发送回UDP套接字。
	SendTo(buf []byte) error
}

func TcpService(callback ITCPEventCallback, addr string, opt TcpOption) error {
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
	if err = ln.initFd(); err != nil {
		return err
	}
	defer ln.close()
	return startTcpService(callback, &ln, &opt)
}

func UdpService(callback IUDPEventCallback, addr string, opt UdpOption) error {
	var (
		ln  udpListener
		err error
	)
	defer ln.close()
	if opt.ReusePort {
		ln.ln, err = internal.ReusePortListenPacket("udp", addr)
	} else {
		ln.ln, err = net.ListenPacket("udp", addr)
	}
	if err != nil {
		return err
	}
	if err = ln.initFd(); err != nil {
		return err
	}
	defer ln.close()
	return startUpdService(callback, &ln, &opt)
}
