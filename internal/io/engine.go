package io

import (
	"github.com/atsegelnyk/memcachex/internal/net"
	"github.com/atsegelnyk/memcachex/internal/types"
	"github.com/atsegelnyk/memcachex/proto"
	"sync"
	"sync/atomic"
)

type Engine struct {
	addr                string
	numEventLoops       int
	numEventLoopSockets int
	ringSize            int
	socketIOBatch       int
	lockOSThread        bool

	rrCounter        uint32
	curNumEventLoops uint32

	mu         sync.Mutex
	eventLoops []*eventLoop
}

func NewEngine(
	addr string,
	numEventLoops,
	numEventLoopSockets,
	ringSize,
	socketIOBatch int,
	lockOSThread bool,
) (*Engine, error) {
	e := &Engine{
		addr:                addr,
		mu:                  sync.Mutex{},
		numEventLoops:       numEventLoops,
		numEventLoopSockets: numEventLoopSockets,
		lockOSThread:        lockOSThread,
		ringSize:            ringSize,
		socketIOBatch:       socketIOBatch,
	}

	for i := 0; i < numEventLoops; i++ {
		err := e.spinupEventLoop(i)
		if err != nil {
			return nil, err
		}
	}

	return e, nil
}

func (e *Engine) Enqueue(req *types.Req) error {
	if len(e.eventLoops) == 0 {
		return proto.ErrNoActiveEventLoop
	}

	return e.enqueue(req)
}

func (e *Engine) enqueue(rq *types.Req) error {
	if e.curNumEventLoops == 0 {
		return proto.ErrBusy
	}

	for i := 0; i < int(e.curNumEventLoops); i++ {
		idx := atomic.AddUint32(&e.rrCounter, 1) - 1
		el := e.eventLoops[idx%e.curNumEventLoops]
		if el.enqueue(rq) {
			return nil
		}
	}

	return proto.ErrBusy
}

func (e *Engine) spinupEventLoop(id int) error {
	el, err := newEventLoop(
		id,
		e.ringSize,
		e.socketIOBatch,
		e.lockOSThread,
		e.onEventLoopError,
		e.onEventLoopSocketError,
	)
	if err != nil {
		return err
	}
	e.curNumEventLoops++

	e.eventLoops = append(e.eventLoops, el)
	for i := 0; i < e.numEventLoopSockets; i++ {
		sock, sockErr := net.NewSocket(e.addr, e.ringSize)
		if sockErr != nil {
			return sockErr
		}
		el.enrollSocket(sock)
	}

	el.start()

	return nil
}

func (e *Engine) onEventLoopError(id int, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	err = e.spinupEventLoop(id)
	if err != nil {
		e.eventLoops = append(e.eventLoops[:id], e.eventLoops[id+1:]...)
		e.curNumEventLoops--
	}
}

func (e *Engine) onEventLoopSocketError(id int, err error) {
	newSock, err := net.NewSocket(e.addr, e.ringSize)
	if err != nil {
		return
	}
	e.eventLoops[id].enrollSocket(newSock)
}
