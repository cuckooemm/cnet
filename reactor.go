package cnet

import (
	"github.com/cuckooemm/cnet/internal/netpoll"
	"golang.org/x/sys/unix"
)

func (srv *tcpServer) activateMainReactor() {
	defer srv.signalShutdown()

	srv.logger.Printf("main reactor exits with error:%v\n", srv.mainLoop.poller.Polling(func(fd int, ev uint32) error {
		return srv.acceptNewConnection(fd)
	}))
}

func (srv *tcpServer) acceptNewConnection(fd int) error {
	var (
		cfd int
		sa  unix.Sockaddr
		el  *eventTcpLoop
		err error
	)
	if cfd, sa, err = unix.Accept(fd); err != nil {
		if err == unix.EAGAIN {
			return nil
		}
		return err
	}
	if err = unix.SetNonblock(cfd, true); err != nil {
		return err
	}
	el = srv.subLoopGroup.next()
	var conn = newTCPConn(cfd, el, sa)
	_ = el.poller.Trigger(func() (err error) {
		if err = el.poller.AddRead(cfd); err != nil {
			return
		}
		el.connections[cfd] = conn
		err = el.loopOpen(conn)
		return
	})
	return nil
}

func (srv *tcpServer) activateSubReactor(el *eventTcpLoop) {
	defer srv.signalShutdown()

	srv.logger.Printf("event-loop:%d exits with error:%v\n", el.idx, el.poller.Polling(func(fd int, ev uint32) error {
		if c, ack := el.connections[fd]; ack {
			switch c.outBuf.IsEmpty() {
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
		return nil
	}))
}
