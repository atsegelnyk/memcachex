package io

import (
	"fmt"
	"github.com/atsegelnyk/memcachex/internal/pool"
	"github.com/atsegelnyk/memcachex/internal/proto/ascii"
	"github.com/atsegelnyk/memcachex/internal/ring"
	"github.com/atsegelnyk/memcachex/internal/types"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"runtime"
	"sync"
)

const (
	maxIOBatch             = 128
	defaultPollTimeoutMs   = 50
	defaultRequestRingSize = 8192
	//defaultRequestRingSize = 262144
)

var (
	WakeupData       = []byte{1}
	ErrNoOpenSockets = errors.New("no open sockets available")
)

type eventFD struct {
	r int
	w int
}

func newEventFD() (*eventFD, error) {
	var fds [2]int
	if err := unix.Pipe(fds[:]); err != nil {
		return nil, err
	}

	_ = unix.SetNonblock(fds[0], true)
	_ = unix.SetNonblock(fds[1], true)

	return &eventFD{
		r: fds[0],
		w: fds[1],
	}, nil
}

type eventLoop struct {
	id            int
	pollTimeoutMs int
	lockOSThread  bool

	ready bool

	mu sync.Mutex

	efd     *eventFD
	pfds    []unix.PollFd
	sockets []*socket

	requestRing *ring.MPSC[*types.Req]

	onErrorHook       func(int, error)
	onSocketErrorHook func(int, error)
}

func newEventLoop(id int, lockOSThread bool, onError, onSocketError func(int, error)) (*eventLoop, error) {
	efd, err := newEventFD()
	if err != nil {
		return nil, err
	}

	pfds := make([]unix.PollFd, 1)
	pfds[0] = unix.PollFd{
		Fd:     int32(efd.r),
		Events: unix.POLLIN,
	}

	return &eventLoop{
		id:                id,
		efd:               efd,
		pfds:              pfds,
		lockOSThread:      lockOSThread,
		mu:                sync.Mutex{},
		pollTimeoutMs:     defaultPollTimeoutMs,
		requestRing:       ring.NewMPSC[*types.Req](defaultRequestRingSize),
		onErrorHook:       onError,
		onSocketErrorHook: onSocketError,
	}, nil
}

func (e *eventLoop) enrollSocket(s *socket) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sockets = append(e.sockets, s)
	e.pfds = append(e.pfds, unix.PollFd{})
}

func (e *eventLoop) enqueue(req *types.Req) bool {
	if !e.ready || len(e.sockets) == 0 {
		return false
	}

	ok := e.requestRing.Push(req)
	if !ok {
		return false
	}
	_, _ = unix.Write(e.efd.w, WakeupData)

	return true
}

func (e *eventLoop) start() {
	e.ready = true
	go e.eventLoop()
}

func (e *eventLoop) eventLoop() {
	if e.lockOSThread {
		runtime.LockOSThread()
	}

	e.ready = true
	for e.ready {
		for _, s := range e.sockets {
			e.dispatchRequests(s)
		}

		e.preparePoller()
		err := e.poll(e.pollTimeoutMs)
		if err != nil {
			e.ready = false
			e.onErrorHook(e.id, err)
			return
		}
	}
}

func (e *eventLoop) preparePoller() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := 0; i < len(e.sockets); i++ {
		sock := e.sockets[i]

		events := int16(unix.POLLIN)
		if sock.wantPollout {
			events |= unix.POLLOUT
		}

		e.pfds[i+1] = unix.PollFd{
			Fd:     int32(sock.fd),
			Events: events,
		}
	}
}

func (e *eventLoop) poll(timeoutMs int) error {
	nReady, err := unix.Poll(e.pfds, timeoutMs)
	if err != nil {
		if err == unix.EINTR {
			return nil
		}
		return err
	}
	if nReady == 0 {
		return nil
	}

	// wakeup sock readable
	if e.pfds[0].Revents&unix.POLLIN != 0 {
		nReady--
		err = e.handleWakeup()
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(e.pfds) && nReady > 0; i++ {
		re := e.pfds[i+1].Revents
		if re == 0 {
			continue
		}

		s := e.sockets[i]
		e.onEvent(re, s, i)
		nReady--
	}

	return nil
}

func (e *eventLoop) handleWakeup() error {
	var buf [64]byte
	for {
		_, err := unix.Read(e.efd.r, buf[:])
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *eventLoop) onEvent(re int16, s *socket, sIdx int) {
	if re&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		err := s.socketError()
		e.onSocketErr(s, sIdx, err)
		return
	}

	if re&unix.POLLIN != 0 {
		if err := e.onReadable(s); err != nil {
			e.onSocketErr(s, sIdx, err)
			return
		}
	}

	if re&unix.POLLOUT != 0 {
		if err := e.onWriteable(s); err != nil {
			e.onSocketErr(s, sIdx, err)
			return
		}
	}
}

func (e *eventLoop) dispatchRequests(s *socket) {
	for i := 0; i < maxIOBatch; i++ {
		rq, ok := e.requestRing.Pop()
		if !ok {
			break
		}

		ok = s.write(rq.Raw)
		if !ok {
			fmt.Println("took but not written")
			break
		}

		ok = s.inflightRing.Push(rq)
		if !ok {
			break
		}
	}
}

func (e *eventLoop) onWriteable(s *socket) error {
	return s.flush()
}

func (e *eventLoop) onReadable(s *socket) (err error) {
	err = s.read()

	for i := 0; i < maxIOBatch; i++ {
		n := ascii.DecodeResponseLen(s.readBuffer[s.rpos:s.readOffset])
		if n == 0 {
			break
		}

		respBuf := pool.BufferPool.GetFor(n)
		copy(respBuf[:n], s.readBuffer[s.rpos:s.rpos+n])
		s.rpos += n

		rq, ok := s.inflightRing.Pop()
		if !ok {
			return ErrEmptyRing
		}

		if rq.CallbackRequest {
			rq.CmdCallback(rq.CallerCallback, respBuf, err)
			pool.ReqPool.Put(rq)
			continue
		}

		rq.RawResp = respBuf
		rq.Done <- struct{}{}
	}

	if s.rpos == s.readOffset {
		s.rpos, s.readOffset = 0, 0
	} else if s.rpos > 0 {
		copy(s.readBuffer[0:], s.readBuffer[s.rpos:s.readOffset])
		s.readOffset -= s.rpos
		s.rpos = 0
	}

	return
}

func (e *eventLoop) onSocketErr(s *socket, sIdx int, err error) {
	fmt.Println("onSocketErr", sIdx, err)

	e.sockets = append(e.sockets[:sIdx], e.sockets[sIdx+1:]...)
	e.pfds = append(e.pfds[:sIdx+1], e.pfds[sIdx+2:]...)
	if len(e.sockets) == 0 {
		e.ready = false
		e.onErrorHook(e.id, ErrNoOpenSockets)
		return
	}

	_ = s.conn.Close()
	e.onSocketErrorHook(e.id, err)
}
