package io

import (
	"github.com/atsegelnyk/memcachex/internal/types"
	"github.com/pkg/errors"
	"sync"
	"sync/atomic"
)

const (
	defaultEventLoopSockets = 2
	defaultEnqueueAttempts  = 3
)

var (
	ErrBusy              = errors.New("engine busy")
	ErrNoActiveEventLoop = errors.New("no active event loop")
	ErrEmptyRing         = errors.New("empty ring")
)

type Engine struct {
	addr          string
	numEventLoops int
	lockOSThread  bool

	rrCounter uint32

	mu         sync.Mutex
	eventLoops []*eventLoop
}

func NewEngine(addr string, numEventLoops int, lockOSThread bool) (*Engine, error) {
	e := &Engine{
		addr:          addr,
		mu:            sync.Mutex{},
		numEventLoops: numEventLoops,
		lockOSThread:  lockOSThread,
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
		return ErrNoActiveEventLoop
	}

	return e.enqueue(req, defaultEnqueueAttempts)
}

func (e *Engine) enqueue(rq *types.Req, attempts int) error {
	n := uint32(len(e.eventLoops))
	if n == 0 {
		return ErrBusy
	}

	for i := 0; i < attempts; i++ {
		idx := atomic.AddUint32(&e.rrCounter, 1) - 1
		el := e.eventLoops[idx%n]
		if el.enqueue(rq) {
			return nil
		}
	}

	return ErrBusy
}

func (e *Engine) spinupEventLoop(id int) error {
	el, err := newEventLoop(id,
		e.lockOSThread,
		e.onEventLoopError,
		e.onEventLoopSocketError,
	)
	if err != nil {
		return err
	}

	e.eventLoops = append(e.eventLoops, el)
	for i := 0; i < defaultEventLoopSockets; i++ {
		sock, sockErr := newSocket(e.addr)
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
	}
}

func (e *Engine) onEventLoopSocketError(id int, err error) {
	newSock, err := newSocket(e.addr)
	if err != nil {
		return
	}
	e.eventLoops[id].enrollSocket(newSock)
}
