package internal

import (
	"github.com/libp2p/go-reuseport"
	"net"
)

func ReusePortListen(protocol, addr string) (net.Listener, error) {
	return reuseport.Listen(protocol, addr)
}
