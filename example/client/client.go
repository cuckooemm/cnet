package main

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"
)

func main() {
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		go func(i int) {
			var id = i
			//tcp(id)
			udp(id)
		}(i)
		wg.Add(1)
	}
	wg.Wait()
}

func tcp(id int) {
	var (
		c       net.Conn
		message []byte
		n       int
		rcv     = make([]byte, 1024)
		err     error
	)
	if c, err = net.Dial("tcp", "127.0.0.1:8000"); err != nil {
		panic(err)
	}
	message = []byte(fmt.Sprintf("i am a client %d\n", id))
	if n, err = c.Read(rcv); err != nil {
		panic(err)
	}
	fmt.Printf("connection[%d]rec len = %d, message = %s\n", id, n, rcv)
	for {
		if n, err = c.Write(message); err != nil {
			panic(err)
		}
		if n, err = c.Read(rcv); err != nil {
			panic(err)
		}
		fmt.Printf("connection[%d]rec len = %d, message = %s\n", id, n, rcv)
		runtime.Gosched()
		time.Sleep(time.Millisecond * 200)
	}
}

func udp(id int) {
	var (
		c       net.Conn
		message []byte
		n       int
		rcv     = make([]byte, 1024)
		err     error
	)
	message = []byte("haha")
	if c, err = net.Dial("udp", "127.0.0.1:8000"); err != nil {
		panic(err)
	}

	for {
		if n, err = c.Write(message); err != nil {
			panic(err)
		}
		fmt.Printf("send len = %d\n", n)
		if n, err = c.Read(rcv); err != nil {
			panic(err)
		}
		fmt.Printf("connection[%d]rec len = %d, message = %s\n", id, n, rcv)
		runtime.Gosched()
		time.Sleep(time.Millisecond * 200)
	}
}
