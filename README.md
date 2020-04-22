# cnet

#### TODO
- http插件
- quic插件
- 易用
- 限流
- ...

#### How to use

```go
go get -u github.com/cuckooemm/cnet
```

TCP
```go
type serverTcpCallback struct {
	connTotal, connected, close int64
	spanDown, spanUp            int64
}
// 链接accept时回调
func (sc *serverTcpCallback) OnConnOpened(c Conn) (out []byte, op Operation) {
	var str = "hello client, welcome to connection\n"
	out = []byte(str)
	atomic.AddInt64(&sc.spanUp, int64(len(out)))
	atomic.AddInt64(&sc.connected, 1)
	atomic.AddInt64(&sc.connTotal, 1)
	fmt.Println("conned - ", sc.connected, "connTotal - ", sc.connTotal, "close - ", sc.close)
	fmt.Println("spanDown - ", sc.spanDown, "spanUp - ", sc.spanUp)
	return
}

// 链接关闭时回调
func (sc *serverTcpCallback) OnConnClosed(c Conn, err error) (op Operation) {
	if err != nil {
		fmt.Println("connection err: ", err)
	}
	atomic.AddInt64(&sc.connected, -1)
	atomic.AddInt64(&sc.close, 1)
	return
}

// 链接可读时回调 
func (sc *serverTcpCallback) ConnHandler(c Conn) (out []byte, op Operation) {
	var rcv = c.Read()
	var l = len(rcv)
	atomic.AddInt64(&sc.spanDown, int64(l))
	fmt.Printf("已收到 %s client 发来长度为: %d 的信息: %s\n", c.RemoteAddr().String(), l, rcv)
	out = []byte("receive ")
	c.ShiftN(l)
	out = append(out, rcv...)
	atomic.AddInt64(&sc.spanUp, int64(len(out)))
	return
}

// 唤醒conn时触发 c.wake()
func (sc *serverTcpCallback) OnWakenHandler(c Conn) (out []byte, op Operation) {
	return nil, None
}

func main(){
    var (
    	call serverTcpCallback
        addr = ":8000"
        err error
    )   
    if err = TcpService(&call, addr, TcpOption{TcpKeepAlive: time.Minute, MultiCore: 4, ReusePort: true}); err != nil{
        // err handler...
    }
}
```
UDP
```go
type serverUdpCallback struct {
	spanDown, spanUp int64
}

func (sc *serverUdpCallback) PackHandler(pack []byte, p Pconn) (out []byte, op Operation) {
	fmt.Printf("receive message :%s of client: %s", pack, p.RemoteAddr())
	atomic.AddInt64(&sc.spanDown, int64(len(pack)))
	out = append(out, []byte("reply: ")...)
	out = append(out, pack...)
	atomic.AddInt64(&sc.spanUp, int64(len(out)))
	return
}

func (sc *serverUdpCallback) SendErr(remoteAddr string, err error) {
	fmt.Printf("send error: %v of client: %s", err, remoteAddr)
}

func main(){
	var (
		c        cnet.Cnet
		err      error
	)
	c = cnet.Cnet{
		Network:      cnet.Tcp,
		Callback:     &serverCallback{},
		Addr:         ":8000",
		MultiCore:    4,
		TcpKeepAlive: time.Minute,
		ReusePort:    true,
	}
	if err = c.Listener(); err != nil {
		println(err)
	}
}
```