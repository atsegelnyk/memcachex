package io

import (
	"github.com/atsegelnyk/memcachex/internal/net"
	"github.com/atsegelnyk/memcachex/internal/netpoll"
	"github.com/atsegelnyk/memcachex/internal/pool"
	"github.com/atsegelnyk/memcachex/internal/proto/ascii"
	"github.com/atsegelnyk/memcachex/internal/ring"
	"github.com/atsegelnyk/memcachex/internal/types"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"runtime"
	"sync/atomic"
)

var (
	ErrNoOpenSockets = errors.New("no open sockets available")
	ErrEmptyRing     = errors.New("empty ring")
)

type eventLoop struct {
	id            int
	socketIOBatch int
	lockOSThread  bool

	ready bool

	socketRRCounter uint32
	sockets         []*net.Socket

	requestRing *ring.MPSC[*types.Req]

	poller *netpoll.Poller

	onErrorHook       func(int, error)
	onSocketErrorHook func(int, error)
}

func newEventLoop(id, ringSize, socketIOBatch int, lockOSThread bool, onError, onSocketError func(int, error)) (*eventLoop, error) {
	e := &eventLoop{
		id:                id,
		lockOSThread:      lockOSThread,
		socketIOBatch:     socketIOBatch,
		requestRing:       ring.NewMPSC[*types.Req](ringSize),
		onErrorHook:       onError,
		onSocketErrorHook: onSocketError,
	}

	poller, err := netpoll.NewPoller(e.onReadable, e.onWriteable, e.onSocketErr)
	if err != nil {
		return nil, err
	}

	e.poller = poller

	return e, nil
}

func (e *eventLoop) enrollSocket(s *net.Socket) error {
	err := e.poller.Add(s)
	if err != nil {
		return err
	}

	// set socket index
	s.ID = len(e.sockets)
	e.sockets = append(e.sockets, s)

	return nil
}

func (e *eventLoop) enqueue(req *types.Req) bool {
	if !e.ready || len(e.sockets) == 0 {
		return false
	}

	ok := e.requestRing.Push(req)
	if !ok {
		return false
	}

	err := e.poller.Wakeup()
	if err != nil {
		e.ready = false
		e.onErrorHook(e.id, err)
		return false
	}

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
		n := uint32(len(e.sockets))
		for i := 0; i < len(e.sockets); i++ {
			idx := atomic.AddUint32(&e.socketRRCounter, 1) - 1
			s := e.sockets[idx%n]

			e.dispatchRequests(s)
			err := e.poller.Mod(s)
			if err != nil {
				e.onErrorHook(e.id, err)
				return
			}
		}

		err := e.poller.Poll()
		if err != nil {
			e.ready = false
			e.onErrorHook(e.id, err)
		}
	}
}

func (e *eventLoop) dispatchRequests(s *net.Socket) {
	for i := 0; i < e.socketIOBatch; i++ {
		if !s.InflightRing.CanPush() {
			return
		}

		// try to peek from requestRing, then try to write to the socket.
		// if succeeds, discard peeked object from request ring, and push to socket's inflight ring.
		rq, ok := e.requestRing.Peek()
		if !ok {
			return
		}

		ok = s.Write(rq.Raw)
		if !ok {
			return
		}

		// return request buffer to bufferPool
		pool.BufferPool.Put(rq.Raw)

		_ = e.requestRing.Discard()
		s.InflightRing.Push(rq)
	}
}

func (e *eventLoop) onWriteable(s *net.Socket) {
	err := s.Flush()
	if err != nil {
		e.onSocketErr(s)
	}
}

func (e *eventLoop) onReadable(s *net.Socket) {
	err := s.Read()

	for i := 0; i < e.socketIOBatch; i++ {
		n := ascii.DecodeResponseLen(s.ReadBuffer[s.Rpos:s.ReadOffset])
		if n == 0 {
			break
		}

		respBuf := pool.BufferPool.GetFor(n)
		copy(respBuf[:n], s.ReadBuffer[s.Rpos:s.Rpos+n])
		s.Rpos += n

		rq, ok := s.InflightRing.Pop()

		if !ok {
			e.onSocketErr(s)
			return
		}

		if rq.NonBlocking {
			rq.CmdCallback(rq.CallerCallback, respBuf, err)
			// return request object to pool
			pool.ReqPool.Put(rq)
			continue
		}

		rq.RawResp = respBuf
		rq.Done <- struct{}{}
	}

	if s.Rpos == s.ReadOffset {
		s.Rpos, s.ReadOffset = 0, 0
	} else if s.Rpos > 0 {
		copy(s.ReadBuffer[0:], s.ReadBuffer[s.Rpos:s.ReadOffset])
		s.ReadOffset -= s.Rpos
		s.Rpos = 0
	}

	return
}

func (e *eventLoop) onSocketErr(s *net.Socket) {
	e.sockets = append(e.sockets[:s.ID], e.sockets[s.ID+1:]...)
	if len(e.sockets) == 0 {
		e.ready = false
		e.onErrorHook(e.id, ErrNoOpenSockets)
		return
	}

	s.Close()
	err := getSocketError(s.FD)
	e.onSocketErrorHook(e.id, err)
}

func getSocketError(fd int) error {
	nerr, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		return err
	}
	if nerr == 0 {
		return nil
	}
	return unix.Errno(nerr)
}
