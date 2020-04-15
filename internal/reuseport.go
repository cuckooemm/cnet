package internal

import (
	"github.com/libp2p/go-reuseport"
	"net"
)

func ReusePortListen(protocol, addr string) (net.Listener, error) {
	return reuseport.Listen(protocol, addr)
}
func ReusePortListenPacket(proto, addr string) (net.PacketConn, error) {
	return reuseport.ListenPacket(proto, addr)
}
