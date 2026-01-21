//go:build linux

package netpoll

import (
	"github.com/atsegelnyk/memcachex/internal/net"
	"golang.org/x/sys/unix"
	"sync/atomic"
	"unsafe"
)

var (
	u uint64 = 1
	b        = (*(*[8]byte)(unsafe.Pointer(&u)))[:]
)

type Poller struct {
	efd        int
	eventFD    int
	eventFDBuf []byte

	nextSleepMsec int
	wakeupCall    int32

	events []unix.EpollEvent

	onSocketReadable  func(*net.Socket)
	onSocketWriteable func(*net.Socket)
	onSocketError     func(*net.Socket)
}

func NewPoller(onSocketReadable, onSocketWriteable, onSocketError func(*net.Socket)) (p *Poller, err error) {
	p = &Poller{
		nextSleepMsec:     -1,
		eventFDBuf:        make([]byte, 8),
		events:            make([]unix.EpollEvent, 8),
		onSocketReadable:  onSocketReadable,
		onSocketWriteable: onSocketWriteable,
		onSocketError:     onSocketError,
	}

	p.eventFD, err = unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		return
	}

	p.efd, err = unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		_ = unix.Close(p.eventFD)
		return
	}

	err = unix.EpollCtl(p.efd, unix.EPOLL_CTL_ADD, p.eventFD, &unix.EpollEvent{
		Fd:     int32(p.eventFD),
		Events: ReadEvents,
	})

	if err != nil {
		_ = unix.Close(p.eventFD)
		_ = unix.Close(p.efd)
	}
	return
}

func (p *Poller) Close() {
	_ = unix.Close(p.eventFD)
	_ = unix.Close(p.efd)
}

func (p *Poller) Wakeup() (err error) {
	if atomic.CompareAndSwapInt32(&p.wakeupCall, 0, 1) {
		_, err = unix.Write(p.eventFD, b)
		if err == unix.EAGAIN {
			_, _ = unix.Read(p.eventFD, p.eventFDBuf)
			_, err = unix.Write(p.eventFD, b)
		}
	}
	return err
}

func (p *Poller) Add(s *net.Socket) error {
	event := setPointerDataToEpollEvent(s, ReadEvents)
	return unix.EpollCtl(p.efd, unix.EPOLL_CTL_ADD, s.FD, event)
}

func (p *Poller) Mod(s *net.Socket) error {
	if s.WantWrite {
		if s.PollerWantWriteState {
			return nil
		}
		s.PollerWantWriteState = true
		event := setPointerDataToEpollEvent(s, ReadWriteEvents)
		return unix.EpollCtl(p.efd, unix.EPOLL_CTL_MOD, s.FD, event)
	}

	if !s.PollerWantWriteState {
		return nil
	}
	s.PollerWantWriteState = false
	event := setPointerDataToEpollEvent(s, ReadEvents)
	return unix.EpollCtl(p.efd, unix.EPOLL_CTL_MOD, s.FD, event)
}

func (p *Poller) Delete(s *net.Socket) error {
	return unix.EpollCtl(p.efd, unix.EPOLL_CTL_DEL, s.FD, nil)
}

func (p *Poller) Poll() error {
	n, err := unix.EpollWait(p.efd, p.events, p.nextSleepMsec)
	if n == 0 || (n < 0 && err == unix.EINTR) {
		atomic.StoreInt32(&p.wakeupCall, 0)
		p.nextSleepMsec = -1
		return nil
	} else if err != nil {
		return err
	}

	p.nextSleepMsec = 0
	atomic.StoreInt32(&p.wakeupCall, 0)

	for i := 0; i < n; i++ {
		ev := &p.events[i]

		if fd := int(ev.Fd); fd == p.eventFD {
			_, _ = unix.Read(p.eventFD, p.eventFDBuf)
			continue
		}

		socket := getPointerDataFromEpollEvent(ev)

		if ev.Events&(unix.EPOLLERR|unix.EPOLLHUP) != 0 {
			p.onSocketError(socket)
			continue
		}

		if ev.Events&unix.EPOLLIN != 0 {
			p.onSocketReadable(socket)
		}

		if ev.Events&unix.EPOLLOUT != 0 {
			p.onSocketWriteable(socket)
		}

	}

	return nil
}
