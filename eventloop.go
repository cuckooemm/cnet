package cnet

import (
	"github.com/cuckooemm/cnet/internal/netpoll"
	"golang.org/x/sys/unix"
	"net"
	"time"
)

type eventTcpLoop struct {
	idx          int               // loop index in the server loops list
	srv          *tcpServer        // server in loop
	buffer       []byte            // read buffer
	poller       *netpoll.Poller   // epoll
	connections  map[int]*conn     // loop connections fd -> conn
	eventHandler ITCPEventCallback // user eventHandler
}

type eventUdpLoop struct {
	idx          int               // loop index in the server loops list
	srv          *udpServer        // server in loop
	buffer       []byte            // read buffer
	poller       *netpoll.Poller   // epoll
	eventHandler IUDPEventCallback // user eventHandler
}

func (el *eventTcpLoop) loopRun() {
	defer el.srv.signalShutdown()
	el.srv.logger.Printf("event-loop: %d: Listener: %s", el.idx, el.srv.ln.ln.Addr().String())
	if err := el.poller.Polling(el.handleEvent); err != nil {
		el.srv.logger.Printf("closeLoops idx = %d error : %s\n", el.idx, err.Error())
	}
}

func (el *eventUdpLoop) loopRun() {
	defer el.srv.signalShutdown()
	el.srv.logger.Printf("event-loop: %d: Listener: %s", el.idx, el.srv.ln.ln.LocalAddr().String())
	if err := el.poller.Polling(el.handleEvent); err != nil {
		el.srv.logger.Printf("closeLoops idx = %d error : %s\n", el.idx, err.Error())
	}
}

func (el *eventTcpLoop) loopAccept(fd int) error {
	if fd == el.srv.ln.fd {
		var (
			cfd int
			sa  unix.Sockaddr
			err error
		)
		cfd, sa, err = unix.Accept(fd)
		if err != nil {
			if err == unix.EAGAIN {
				return nil
			}
			return err
		}
		if err = unix.SetNonblock(cfd, true); err != nil {
			return err
		}
		var conn = newTCPConn(cfd, el, sa)
		// 注册事件
		if err = el.poller.AddRead(conn.fd); err != nil {
			return err
		}
		el.connections[conn.fd] = conn
		return el.loopOpen(conn)
	}
	return nil
}

func (el *eventTcpLoop) loopOpen(c *conn) error {
	c.opened = true
	out, action := el.eventHandler.OnConnOpened(c)
	if el.srv.opt.TcpKeepAlive > 0 {
		if _, ok := el.srv.ln.ln.(*net.TCPListener); ok {
			_ = netpoll.SetKeepAlive(c.fd, int(el.srv.opt.TcpKeepAlive/time.Second))
		}
	}
	if out != nil {
		c.open(out)
	}
	if !c.outBuf.IsEmpty() {
		_ = el.poller.AddWrite(c.fd)
	}
	return el.handleOperation(c, action)
}
func (el *eventTcpLoop) loopRead(c *conn) error {
	var (
		n   int
		out []byte
		op  Operation
		err error
	)
	if n, err = unix.Read(c.fd, el.buffer); n == 0 || err != nil {
		if err == unix.EAGAIN {
			return nil
		}
		return el.loopCloseConn(c, err)
	}
	if _, err = c.inBuf.Write(el.buffer[:n]); err != nil {
		return el.loopCloseConn(c, err)
	}
	out, op = el.eventHandler.ConnHandler(c)
	c.write(out)
	switch op {
	case None:
	case Close:
		return el.loopCloseConn(c, nil)
	case Shutdown:
		return ErrServerShutdown
	}
	return nil
}

func (el *eventTcpLoop) loopWrite(c *conn) error {
	//el.eventHandler.PreWrite()
	var (
		head, tail []byte
		n          int
		err        error
	)
	head, tail = c.outBuf.LazyReadAll()
	if n, err = unix.Write(c.fd, head); err != nil {
		if err == unix.EAGAIN {
			return nil
		}
		return el.loopCloseConn(c, err)
	}
	c.outBuf.Shift(n)

	if len(head) == n && tail != nil {
		if n, err = unix.Write(c.fd, tail); err != nil {
			if err == unix.EAGAIN {
				return nil
			}
			return el.loopCloseConn(c, err)
		}
		c.outBuf.Shift(n)
	}

	if c.outBuf.IsEmpty() {
		if err = el.poller.ModRead(c.fd); err != nil {
			return el.loopCloseConn(c, err)
		}
	}
	return nil
}

func (el *eventTcpLoop) loopCloseConn(c *conn, err error) error {
	if errDel, errClose := el.poller.Delete(c.fd), unix.Close(c.fd); errDel == nil && errClose == nil {
		delete(el.connections, c.fd)
		switch el.eventHandler.OnConnClosed(c, err) {
		case Shutdown:
			return ErrServerShutdown
		}
		c.releaseTCP()
	} else {
		if errDel != nil {
			el.srv.logger.Printf("failed to delete fd: %d from poller: %d, error: %v\n", c.fd, el.idx, errDel)
		}
		if errClose != nil {
			el.srv.logger.Printf("failed to close fd: %d, error:%v\n", c.fd, errClose)
		}
	}
	return nil
}

func (el *eventTcpLoop) loopWake(c *conn) error {
	var (
		out []byte
		op  Operation
	)
	out, op = el.eventHandler.OnWakenHandler(c)
	if out != nil {
		c.write(out)
	}
	return el.handleOperation(c, op)
}

func (el *eventTcpLoop) handleEvent(fd int, ev uint32) error {
	if c, ok := el.connections[fd]; ok {
		switch c.outBuf.IsEmpty() {
		// Don't change the ordering of processing EPOLLOUT | EPOLLRDHUP / EPOLLIN unless you're 100%
		// sure what you're doing!
		// Re-ordering can easily introduce bugs and bad side-effects, as I found out painfully in the past.
		case false:
			if ev&netpoll.OutEvents != 0 {
				return el.loopWrite(c)
			}
			return nil
		case true:
			if ev&netpoll.InEvents != 0 {
				return el.loopRead(c)
			}
			return nil
		}
	}
	return el.loopAccept(fd)
}

func (el *eventUdpLoop) handleEvent(fd int, ev uint32) error {
	if fd == el.srv.ln.fd {
		return el.loopRead(fd)
	}
	return nil
}

func (el *eventUdpLoop) loopRead(fd int) error {
	var (
		n   int
		sa  unix.Sockaddr
		err error
	)
	n, sa, err = unix.Recvfrom(fd, el.buffer, 0)
	if err != nil || n == 0 {
		if err != nil && err != unix.EAGAIN {
			el.srv.logger.Printf("failed to read UPD packet from fd:%d, error:%v\n", fd, err)
		}
		return nil
	}
	var (
		p   *pack
		out []byte
		op  Operation
	)
	p = newUDPPack(fd, el, sa)
	out, op = el.eventHandler.PackHandler(el.buffer[:n], p)
	if out != nil {
		if err = p.sendTo(out); err != nil {
			el.eventHandler.SendErr(p.remoteAddr, err)
		}
	}
	switch op {
	case Shutdown:
		return ErrServerShutdown
	}
	p.releaseUDP()
	return nil
}
func (el *eventTcpLoop) handleOperation(c *conn, op Operation) error {
	switch op {
	case None:
		return nil
	case Close:
		_ = el.loopWrite(c)
		return el.loopCloseConn(c, nil)
	case Shutdown:
		_ = el.loopWrite(c)
		return ErrServerShutdown
	default:
		return nil
	}
}
