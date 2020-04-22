package cnet

import "errors"

var (
	ErrServerShutdown    = errors.New("service is going to be shutdown")
	ErrUnSupportProtocol = errors.New("unsupported protocol")
)
