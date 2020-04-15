package cnet

import "time"

type TcpOption struct {
	ReusePort    bool
	MultiCore    int
	Logger       Logger
	TcpKeepAlive time.Duration
}

type UdpOption struct {
	ReusePort bool
	MultiCore int
	Logger    Logger
}
