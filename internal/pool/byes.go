package pool

import (
	"sync"
)

const (
	XSmallBufferSize  = 1024
	SmallBufferSize   = 4096
	MediumBufferSize  = 8192
	LargeBufferSize   = 16384
	XLargeBufferSize  = 131072
	XXLargeBufferSize = 262144
)

var BufferPool = newBufferPool()

type bufferPool struct {
	xs  *sync.Pool
	s   *sync.Pool
	m   *sync.Pool
	l   *sync.Pool
	xl  *sync.Pool
	xxl *sync.Pool
}

func newBufferPool() *bufferPool {
	xSmallPool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, XSmallBufferSize)
		},
	}
	smallPool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, SmallBufferSize)
		},
	}
	mediumPool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, MediumBufferSize)
		},
	}
	largePool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, LargeBufferSize)
		},
	}
	xLargePool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, XLargeBufferSize)
		},
	}
	xxLargePool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, XXLargeBufferSize)
		},
	}
	return &bufferPool{
		xs:  xSmallPool,
		s:   smallPool,
		m:   mediumPool,
		l:   largePool,
		xl:  xLargePool,
		xxl: xxLargePool,
	}
}

func (p *bufferPool) GetFor(size int) []byte {
	switch {
	case size <= XSmallBufferSize:
		return p.xs.Get().([]byte)[:size]
	case size <= SmallBufferSize:
		return p.s.Get().([]byte)[:size]
	case size <= MediumBufferSize:
		return p.m.Get().([]byte)[:size]
	case size <= LargeBufferSize:
		return p.l.Get().([]byte)[:size]
	case size <= XLargeBufferSize:
		return p.xl.Get().([]byte)[:size]
	case size <= XXLargeBufferSize:
		return p.xxl.Get().([]byte)[:size]
	default:
		return make([]byte, size)
	}
}

func (p *bufferPool) Put(b []byte) {
	c := cap(b)
	b = b[:0]

	switch c {
	case XSmallBufferSize:
		p.xs.Put(b)
	case SmallBufferSize:
		p.s.Put(b)
	case MediumBufferSize:
		p.m.Put(b)
	case LargeBufferSize:
		p.l.Put(b)
	case XLargeBufferSize:
		p.xl.Put(b)
	case XXLargeBufferSize:
		p.xxl.Put(b)
	}
}
