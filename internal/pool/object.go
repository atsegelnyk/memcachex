package pool

import (
	"github.com/atsegelnyk/memcachex/internal/types"
	"sync"
)

var FuturePool = sync.Pool{
	New: func() interface{} {
		return make(chan struct{}, 1)
	},
}

var ReqPool = sync.Pool{
	New: func() interface{} {
		return &types.Req{
			Done: make(chan struct{}, 1),
		}
	},
}
