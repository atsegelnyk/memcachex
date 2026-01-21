//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package netpoll

import (
	"github.com/atsegelnyk/memcachex/internal/net"
	"os"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
)

type Poller struct {
	fd int

	nonBlock *unix.Timespec

	nextSleepTimeSpec *unix.Timespec
	wakeupCall        int32

	events []unix.Kevent_t

	onSocketReadable  func(*net.Socket)
	onSocketWriteable func(*net.Socket)
	onSocketError     func(*net.Socket)
}

func NewPoller(onSocketReadable, onSocketWriteable, onSocketError func(*net.Socket)) (p *Poller, err error) {
	p = &Poller{
		nextSleepTimeSpec: nil,
		nonBlock:          &unix.Timespec{},
		events:            make([]unix.Kevent_t, 16),
		onSocketReadable:  onSocketReadable,
		onSocketWriteable: onSocketWriteable,
		onSocketError:     onSocketError,
	}
	if p.fd, err = unix.Kqueue(); err != nil {
		return
	}

	_, err = unix.Kevent(p.fd, []unix.Kevent_t{{
		Ident:  0,
		Filter: unix.EVFILT_USER,
		Flags:  unix.EV_ADD | unix.EV_CLEAR,
	}}, nil, nil)

	return
}

func (p *Poller) Wakeup() (err error) {
	if atomic.CompareAndSwapInt32(&p.wakeupCall, 0, 1) {
		for {
			_, err = unix.Kevent(p.fd, []unix.Kevent_t{{
				Ident:  0,
				Filter: unix.EVFILT_USER,
				Fflags: unix.NOTE_TRIGGER,
			}}, nil, nil)
			if err == nil {
				return nil
			}
			if err == unix.EINTR {
				continue
			}
			return
		}

	}
	return
}

func (p *Poller) Add(s *net.Socket) error {
	kev := unix.Kevent_t{
		Ident:  uint64(s.FD),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_ADD | unix.EV_ENABLE,
		Udata:  (*byte)(unsafe.Pointer(s)),
	}

	_, err := unix.Kevent(p.fd, []unix.Kevent_t{kev}, nil, nil)
	if err != nil {
		return os.NewSyscallError("kevent add read", err)
	}
	return nil
}

func (p *Poller) Mod(s *net.Socket) error {
	if s.WantWrite {
		if s.PollerWantWriteState {
			return nil
		}
		s.PollerWantWriteState = true

		kev := unix.Kevent_t{
			Ident:  uint64(s.FD),
			Filter: unix.EVFILT_WRITE,
			Flags:  unix.EV_ADD | unix.EV_ENABLE,
			Udata:  (*byte)(unsafe.Pointer(s)),
		}
		_, err := unix.Kevent(p.fd, []unix.Kevent_t{kev}, nil, nil)
		return err
	}

	if !s.PollerWantWriteState {
		return nil
	}
	s.PollerWantWriteState = false

	kev := unix.Kevent_t{
		Ident:  uint64(s.FD),
		Filter: unix.EVFILT_WRITE,
		Flags:  unix.EV_DELETE,
	}
	_, err := unix.Kevent(p.fd, []unix.Kevent_t{kev}, nil, nil)
	return err
}

func (p *Poller) Delete(_ *net.Socket) error {
	return nil
}

func (p *Poller) Poll() error {
	n, err := unix.Kevent(p.fd, nil, p.events, p.nextSleepTimeSpec)
	if n == 0 || (n < 0 && err == unix.EINTR) {
		atomic.StoreInt32(&p.wakeupCall, 0)
		p.nextSleepTimeSpec = nil
		return nil
	} else if err != nil {
		return err
	}

	p.nextSleepTimeSpec = p.nonBlock
	atomic.StoreInt32(&p.wakeupCall, 0)

	for i := 0; i < n; i++ {
		ev := &p.events[i]
		if fd := int(ev.Ident); fd == 0 {
			continue
		}

		socket := (*net.Socket)(unsafe.Pointer(ev.Udata))

		if ev.Flags&unix.EV_ERROR != 0 {
			p.onSocketError(socket)
			continue
		}

		switch ev.Filter {
		case unix.EVFILT_READ:
			p.onSocketReadable(socket)
		case unix.EVFILT_WRITE:
			p.onSocketWriteable(socket)
		}

		if ev.Flags&unix.EV_EOF != 0 {
			p.onSocketError(socket)
		}
	}

	return nil
}
