package types

import (
	"github.com/atsegelnyk/memcachex/proto"
)

type Req struct {
	Raw     []byte
	RawResp []byte

	TS int64

	NonBlocking bool
	Done        chan struct{}

	CallerCallback proto.CallerCallback
	CmdCallback    func(proto.CallerCallback, []byte, error)
}
