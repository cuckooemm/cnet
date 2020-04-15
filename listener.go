package cnet

import (
	"golang.org/x/sys/unix"
	"net"
	"os"
	"sync"
)

type tcpListener struct {
	f    *os.File
	fd   int
	ln   net.Listener
	once sync.Once
}

type udpListener struct {
	f    *os.File
	fd   int
	ln   net.PacketConn
	once sync.Once
}

func (ln *tcpListener) initFd() error {
	var err error
	if ln.f, err = ln.ln.(*net.TCPListener).File(); err != nil {
		ln.close()
		return err
	}
	ln.fd = int(ln.f.Fd())
	// 设置非阻塞
	return unix.SetNonblock(ln.fd, true)
}

func (ln *udpListener) initFd() error {
	var err error
	switch pconn := ln.ln.(type) {
	case *net.UDPConn:
		ln.f, err = pconn.File()
	}
	if err != nil {
		ln.close()
		panic(err)
	}
	ln.fd = int(ln.f.Fd())
	// 设置非阻塞
	return unix.SetNonblock(ln.fd, true)
}

func (ln *udpListener) close() {
	ln.once.Do(func() {
		var err error
		if ln.f != nil {
			if err = ln.f.Close(); err != nil {
				defaultLogger.Printf("udpListener fd close error: %v\n", err)
			}
		}
		if ln.ln != nil {
			if err = ln.ln.Close(); err != nil {
				defaultLogger.Printf("udpListener fd close error: %v\n", err)
			}
		}
	})
}

func (ln *tcpListener) close() {
	ln.once.Do(func() {
		var err error
		if ln.f != nil {
			if err = ln.f.Close(); err != nil {
				defaultLogger.Printf("tcpListener fd close error: %v\n", err)
			}
		}
		if ln.ln != nil {
			if err = ln.ln.Close(); err != nil {
				defaultLogger.Printf("tcpListener fd close error : %v\n", err)
			}
		}
	})
}
