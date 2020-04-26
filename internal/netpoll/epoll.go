package netpoll

import (
	"github.com/cuckooemm/cnet/internal/asyncwork"
	"golang.org/x/sys/unix"
	"log"
	"unsafe"
)

const (
	readEvents      = unix.EPOLLPRI | unix.EPOLLIN
	writeEvents     = unix.EPOLLOUT
	readWriteEvents = readEvents | writeEvents
)

type Poller struct {
	efd       int    // epoll fd
	wfd       int    // wake fd
	wfdBuf    []byte // wfd buffer to read byte
	asyncWork asyncwork.Queue
}

// CreatePoller instantiates a poller.
func CreatePoller() (*Poller, error) {
	var (
		pr  *Poller
		err error
	)
	pr = new(Poller)
	if pr.efd, err = unix.EpollCreate1(unix.EPOLL_CLOEXEC); err != nil {
		return nil, err
	}
	if pr.wfd, err = unix.Eventfd(0, unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return nil, err
	}
	pr.wfdBuf = make([]byte, 8)
	if err = pr.AddRead(pr.wfd); err != nil {
		return nil, err
	}
	pr.asyncWork = asyncwork.NewQueue()
	return pr, nil
}

func (p *Poller) Polling(callback func(fd int, ev uint32) error) (err error) {
	var eventList = newEventList(InitEvents)
	var waken bool
	for {
		var n int
		if n, err = unix.EpollWait(p.efd, eventList.events, -1); err != nil && err != unix.EINTR {
			log.Println(err)
			continue
		}
		for i := 0; i < n; i++ {
			if fd := int(eventList.events[i].Fd); fd != p.wfd {
				if err = callback(fd, eventList.events[i].Events); err != nil {
					return
				}
			} else {
				waken = true
				_, _ = unix.Read(p.wfd, p.wfdBuf)
			}
		}
		// wfd触发唤醒执行异步任务
		if waken {
			waken = false
			if err = p.asyncWork.Exec(); err != nil {
				return
			}
		}
		if n == eventList.size {
			eventList.increase()
		}
	}
}

func (p *Poller) Close() error {
	if err := unix.Close(p.wfd); err != nil {
		return err
	}
	return unix.Close(p.efd)
}

// Make the endianness of bytes compatible with more linux OSs under different processor-architectures,
// according to http://man7.org/linux/man-pages/man2/eventfd.2.html.
var (
	u uint64 = 1
	b        = (*(*[8]byte)(unsafe.Pointer(&u)))[:]
)

func (p *Poller) Trigger(w asyncwork.Work) error {
	if p.asyncWork.Add(w) == 1 {
		_, err := unix.Write(p.wfd, b)
		return err
	}
	return nil
}

// AddRead registers the given file-descriptor with readable event to the poller.
func (p *Poller) AddRead(fd int) error {
	return unix.EpollCtl(p.efd, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Fd: int32(fd), Events: readEvents})
}

func (p *Poller) AddWrite(fd int) error {
	return unix.EpollCtl(p.efd, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Fd: int32(fd), Events: writeEvents})
}

func (p *Poller) ModReadWrite(fd int) error {
	return unix.EpollCtl(p.efd, unix.EPOLL_CTL_MOD, fd, &unix.EpollEvent{Fd: int32(fd), Events: readWriteEvents})
}

func (p *Poller) ModRead(fd int) error {
	return unix.EpollCtl(p.efd, unix.EPOLL_CTL_MOD, fd, &unix.EpollEvent{Fd: int32(fd), Events: readEvents})
}

func (p *Poller) Delete(fd int) error {
	return unix.EpollCtl(p.efd, unix.EPOLL_CTL_DEL, fd, nil)
}

// SetKeepAlive sets the keepalive for the connection.
func SetKeepAlive(fd, secs int) error {
	var err error
	if err = unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_KEEPALIVE, 1); err != nil {
		return err
	}
	if err = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, secs); err != nil {
		return err
	}
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPIDLE, secs)
}
