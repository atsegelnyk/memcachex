package pool

import (
	"github.com/atsegelnyk/memcachex/internal/types"
	"sync"
)

var ReqPool = sync.Pool{
	New: func() interface{} {
		return &types.Req{
			Done: make(chan struct{}, 1),
		}
	},
}
