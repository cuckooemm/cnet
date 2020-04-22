package cnet

import (
	"github.com/cuckooemm/cnet/internal/netpoll"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
)

var defaultLogger = Logger(log.New(os.Stderr, "[service] - ", log.LstdFlags))

type tcpServer struct {
	ln                 *tcpListener
	wg                 sync.WaitGroup // event-loop close WaitGroup
	opt                *TcpOption     // options with server
	once               sync.Once      // make sure only signalShutdown once
	cond               *sync.Cond     // shutdown signaler
	logger             Logger         // customized logger for logging info
	network, localAddr string         // network and local address
	loop               *eventTcpLoop  // main loop for accepting connections
	mainLoop           *eventTcpLoop
	eventHandler       IEventCallback     // user eventHandler
	subLoopGroup       IEventTcpLoopGroup // loops for handling events
}
type udpServer struct {
	ln                 *udpListener
	wg                 sync.WaitGroup // event-loop close WaitGroup
	opt                *UdpOption     // options with server
	once               sync.Once      // make sure only signalShutdown once
	cond               *sync.Cond     // shutdown signaler
	logger             Logger         // customized logger for logging info
	network, localAddr string         // network and local address
	loop               *eventUdpLoop
	loopGroup          []*eventUdpLoop
	eventHandler       IEventCallback // user eventHandler
}

// 开启服务
func (srv *tcpServer) start(core int) error {
	// 内核负载均衡
	if srv.opt.ReusePort {
		return srv.initLoops(core)
	}
	return srv.initReactors(core)
}

// 等待服务关闭
func (srv *tcpServer) wait() {
	go srv.signalHandler()
	// Wait on a signal for shutdown
	srv.waitForShutdown()
	var err error
	// 通知loop关闭监听
	srv.subLoopGroup.iterate(func(el *eventTcpLoop) bool {
		if err = el.poller.Trigger(func() error {
			return ErrServerShutdown
		}); err != nil {
			srv.logger.Printf("failed to close %d event-loop. err : %v\n", el.idx, err)
		}
		return true
	})

	if srv.mainLoop != nil {
		srv.ln.close()
		if err = srv.mainLoop.poller.Trigger(func() error {
			return ErrServerShutdown
		}); err != nil {
			srv.logger.Printf("failed to close main-loop. err : %v\n", err)
		}
	}

	// Wait on all loops to complete reading events
	srv.wg.Wait()

	// Close loops and all outstanding connections
	srv.subLoopGroup.iterate(func(el *eventTcpLoop) bool {
		for _, c := range el.connections {
			if err := el.loopCloseConn(c, nil); err != nil {
				srv.logger.Printf("failed to close connection %s\n", c.remoteAddr)
			}
		}
		return true
	})
	srv.closeLoops()

	if srv.mainLoop != nil {
		if err = srv.mainLoop.poller.Close(); err != nil {
			srv.logger.Printf("failed to close main-loop poller,err : %v\n", err)
		}
	}
}

func (srv *udpServer) stop() {
	// Wait on a signal for shutdown
	srv.waitForShutdown()
	var err error
	// 通知loop关闭监听
	for _, loop := range srv.loopGroup {
		if err = loop.poller.Trigger(func() error {
			return ErrServerShutdown
		}); err != nil {
			srv.logger.Printf("failed to close %d event-loop. err : %v\n", loop.idx, err)
		}
	}
	// Wait on all loops to complete reading events
	srv.wg.Wait()

	srv.closeLoops()
}

func (srv *tcpServer) initLoops(core int) error {
	for i := 0; i < core; i++ {
		var (
			pr  *netpoll.Poller
			el  *eventTcpLoop
			err error
		)
		if pr, err = netpoll.CreatePoller(); err != nil {
			return err
		}
		el = &eventTcpLoop{
			idx:          i,
			srv:          srv,
			poller:       pr,
			buffer:       make([]byte, 0x10000), // 65536
			connections:  make(map[int]*conn, 16),
			eventHandler: srv.eventHandler,
		}
		// event-loop监听同一fd 监听fd事件到达时会唤醒全部 TODO 这里可以监听多端口
		if err = el.poller.AddRead(srv.ln.fd); err != nil {
			return err
		}
		srv.subLoopGroup.register(el)
	}
	srv.startLoops()
	return nil
}

func (srv *udpServer) initLoops(core int) error {
	srv.wg.Add(core)
	for i := 0; i < core; i++ {
		var (
			pr  *netpoll.Poller
			el  *eventUdpLoop
			err error
		)
		if pr, err = netpoll.CreatePoller(); err != nil {
			return err
		}
		el = &eventUdpLoop{
			idx:          i,
			srv:          srv,
			poller:       pr,
			buffer:       make([]byte, 0x10000), // 65536
			eventHandler: srv.eventHandler,
		}
		// event-loop监听同一fd 监听fd事件到达时会唤醒全部 TODO 这里可以监听多端口
		if err = el.poller.AddRead(srv.ln.fd); err != nil {
			return err
		}
		go func(el *eventUdpLoop) {
			el.loopRun()
			srv.wg.Done()
		}(el)
	}
	return nil
}

func (srv *tcpServer) startLoops() {
	srv.subLoopGroup.iterate(func(loop *eventTcpLoop) bool {
		srv.wg.Add(1)
		go func() {
			loop.loopRun()
			srv.wg.Done()
		}()
		return true
	})
}

func (srv *tcpServer) initReactors(numEventLoop int) error {
	for i := 0; i < numEventLoop; i++ {
		var (
			pr  *netpoll.Poller
			err error
		)
		if pr, err = netpoll.CreatePoller(); err == nil {
			el := &eventTcpLoop{
				idx:          i,
				srv:          srv,
				poller:       pr,
				buffer:       make([]byte, 0x10000), // 65536
				connections:  make(map[int]*conn),
				eventHandler: srv.eventHandler,
			}
			srv.subLoopGroup.register(el)
		} else {
			return err
		}
	}
	// Start sub reactors.
	srv.startReactors()
	var (
		pr  *netpoll.Poller
		el  *eventTcpLoop
		err error
	)
	if pr, err = netpoll.CreatePoller(); err != nil {
		return err
	}
	el = &eventTcpLoop{
		idx:    -1,
		poller: pr,
		srv:    srv,
	}
	if err = el.poller.AddRead(srv.ln.fd); err != nil {
		return err
	}
	srv.mainLoop = el
	srv.wg.Add(1)
	go func() {
		srv.activateMainReactor()
		srv.wg.Done()
	}()
	return nil
}

func (srv *tcpServer) startReactors() {
	srv.subLoopGroup.iterate(func(el *eventTcpLoop) bool {
		srv.wg.Add(1)
		go func() {
			srv.activateSubReactor(el)
			srv.wg.Done()
		}()
		return true
	})
}

func (srv *tcpServer) closeLoops() {
	srv.subLoopGroup.iterate(func(loop *eventTcpLoop) bool {
		if err := loop.poller.Close(); err != nil {
			srv.logger.Printf("closeLoops idx = %d error : %s\n", loop.idx, err.Error())
		}
		return true
	})
}

func (srv *udpServer) closeLoops() {
	for _, loop := range srv.loopGroup {
		if err := loop.poller.Close(); err != nil {
			srv.logger.Printf("closeLoops idx = %d error : %s\n", loop.idx, err.Error())
		}
	}
}

func (srv *tcpServer) signalShutdown() {
	srv.once.Do(func() {
		srv.cond.L.Lock()
		srv.cond.Signal()
		srv.cond.L.Unlock()
	})
}
func (srv *tcpServer) waitForShutdown() {
	srv.cond.L.Lock()
	srv.cond.Wait()
	srv.cond.L.Unlock()
}
func (srv *udpServer) signalShutdown() {
	srv.once.Do(func() {
		srv.cond.L.Lock()
		srv.cond.Signal()
		srv.cond.L.Unlock()
	})
}
func (srv *udpServer) waitForShutdown() {
	srv.cond.L.Lock()
	srv.cond.Wait()
	srv.cond.L.Unlock()
}

func startTcpService(callback IEventCallback, ln *tcpListener, opt *TcpOption) error {
	var (
		srv tcpServer
		err error
	)
	if opt.MultiCore == 0 {
		opt.MultiCore = runtime.NumCPU()
	}
	srv.opt = opt
	srv.ln = ln
	srv.localAddr = ln.ln.Addr().String()
	srv.network = "tcp"
	srv.eventHandler = callback
	srv.cond = sync.NewCond(&sync.Mutex{})
	srv.subLoopGroup = new(roundRobinEventLoopGroup)
	srv.logger = func() Logger {
		if opt.Logger == nil {
			return defaultLogger
		}
		return opt.Logger
	}()

	if err = srv.start(opt.MultiCore); err != nil {
		srv.closeLoops()
		srv.logger.Printf("service is stop with error : %v\n", err)
		return err
	}
	defer srv.wait()
	return nil
}

func startUpdService(callback IEventCallback, ln *udpListener, opt *UdpOption) error {
	var (
		srv udpServer
		err error
	)
	if opt.MultiCore == 0 {
		opt.MultiCore = runtime.NumCPU()
	}
	srv.opt = opt
	srv.ln = ln
	srv.network = "udp"
	srv.localAddr = ln.ln.LocalAddr().String()
	srv.eventHandler = callback
	srv.cond = sync.NewCond(&sync.Mutex{})
	srv.logger = func() Logger {
		if opt.Logger == nil {
			return defaultLogger
		}
		return opt.Logger
	}()
	if err = srv.initLoops(opt.MultiCore); err != nil {
		srv.closeLoops()
		srv.logger.Printf("service is stop with error : %v\n", err)
		return err
	}
	defer srv.stop()
	return nil
}

func (srv *tcpServer) signalHandler() {
	control := make(chan os.Signal)
	signal.Notify(control, os.Interrupt, os.Kill)
	<-control
	srv.signalShutdown()
}
