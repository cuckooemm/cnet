package main

import (
	"fmt"
	"net"
	"runtime"
	"sync"
)

func main() {
	wg := sync.WaitGroup{}
	for i := 0; i < 1000; i++ {
		go func(id int) {
			connect(i)
		}(i)
		wg.Add(1)
	}
	wg.Wait()
}

func connect(id int) {
	var (
		c       net.Conn
		message []byte
		n       int
		rcv     []byte
		err     error
	)
	if c, err = net.Dial("tcp", "127.0.0.1:8000"); err != nil {
		panic(err)
	}
	message = []byte(fmt.Sprintf("i am a client %d", id))
	if n, err = c.Read(rcv); err != nil {
		panic(err)
	}
	fmt.Printf("rec len = %d, message = %s", n, rcv)
	for {
		if n, err = c.Write(message); err != nil {
			panic(err)
		}
		if n, err = c.Read(rcv); err != nil {
			panic(err)
		}
		runtime.Gosched()
	}
}
