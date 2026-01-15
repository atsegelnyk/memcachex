package memcachex

import (
	"github.com/atsegelnyk/memcachex/internal/io"
	"github.com/atsegelnyk/memcachex/internal/pool"
	"github.com/atsegelnyk/memcachex/internal/proto/ascii"
	"github.com/atsegelnyk/memcachex/internal/types"
	"github.com/atsegelnyk/memcachex/proto"
	"time"
)

const (
	defaultLockOSThread  = false
	defaultNumEventLoops = 1
	defaultAddr          = "localhost:11211"
)

type Client struct {
	addr          string
	lockOSThread  bool
	numEventLoops int

	engine *io.Engine
}

type ClientOption func(*Client)

func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		addr:          defaultAddr,
		lockOSThread:  defaultLockOSThread,
		numEventLoops: defaultNumEventLoops,
	}

	for _, opt := range opts {
		opt(c)
	}

	engine, err := io.NewEngine(c.addr, c.numEventLoops, c.lockOSThread)
	if err != nil {
		return nil, err
	}

	c.engine = engine
	return c, nil
}

func WithNumEventLoops(num int) ClientOption {
	return func(c *Client) {
		c.numEventLoops = num
	}
}

func WithLockOSThread(lock bool) ClientOption {
	return func(c *Client) {
		c.lockOSThread = lock
	}
}

func WithAddr(addr string) ClientOption {
	return func(c *Client) {
		c.addr = addr
	}
}

func (c *Client) Get(key []byte) (*proto.Value, error) {
	raw := ascii.EncodeGet(key)
	resp, err := c.roundTrip(raw)
	if err != nil {
		return nil, err
	}

	getResp, err := ascii.DecodeGetResponse(resp)
	pool.BufferPool.Put(resp)

	return getResp, err
}

func (c *Client) GetAsync(key []byte, callerCb proto.CallerCallback) error {
	raw := ascii.EncodeGet(key)

	req := pool.ReqPool.Get().(*types.Req)
	req.Raw = raw
	req.CmdCallback = getCallback
	req.CallerCallback = callerCb
	req.CallbackRequest = true
	req.TS = time.Now().Unix()

	return c.engine.Enqueue(req)
}

func getCallback(callerCb proto.CallerCallback, r []byte, e error) {
	defer pool.BufferPool.Put(r)
	if e != nil {
		callerCb(nil, e)
		return
	}

	resp, err := ascii.DecodeGetResponse(r)
	callerCb(resp, err)
}

func (c *Client) Set(it *proto.Item) error {
	raw := ascii.EncodeSet(it.Key, it.Flags, it.Expiration, it.Value)
	resp, err := c.roundTrip(raw)
	if err != nil {
		return err
	}

	err = ascii.DecodeSetResponse(resp)
	pool.BufferPool.Put(resp)

	return err
}

func (c *Client) SetAsync(it *proto.Item, callerCb proto.CallerCallback) error {
	raw := ascii.EncodeSet(it.Key, it.Flags, it.Expiration, it.Value)

	req := pool.ReqPool.Get().(*types.Req)
	req.Raw = raw
	req.CmdCallback = setCallback
	req.CallerCallback = callerCb
	req.CallbackRequest = true
	req.TS = time.Now().Unix()

	return c.engine.Enqueue(req)
}

func setCallback(callerCb proto.CallerCallback, r []byte, e error) {
	defer pool.BufferPool.Put(r)
	if e != nil {
		callerCb(nil, e)
		return
	}

	err := ascii.DecodeSetResponse(r)
	callerCb(nil, err)
}

func (c *Client) Version() ([]byte, error) {
	raw := ascii.EncodeVersion()

	rawResp, err := c.roundTrip(raw)
	if err != nil {
		return nil, err
	}

	version, err := ascii.DecodeVersionResponse(rawResp)
	pool.BufferPool.Put(rawResp)

	return version, err
}

func (c *Client) VersionAsync(callerCb proto.CallerCallback) error {
	raw := ascii.EncodeVersion()

	req := pool.ReqPool.Get().(*types.Req)
	req.Raw = raw
	req.CmdCallback = versionCallback
	req.CallerCallback = callerCb
	req.CallbackRequest = true
	req.TS = time.Now().Unix()

	return c.engine.Enqueue(req)
}

func versionCallback(callerCb proto.CallerCallback, r []byte, e error) {
	defer pool.BufferPool.Put(r)
	if e != nil {
		callerCb(nil, e)
		return
	}

	ver, err := ascii.DecodeVersionResponse(r)
	callerCb(ver, err)
}

func (c *Client) roundTrip(raw []byte) (resp []byte, err error) {
	req := pool.ReqPool.Get().(*types.Req)
	defer pool.ReqPool.Put(req)

	req.Raw = raw
	req.TS = time.Now().Unix()
	req.CallbackRequest = false

	err = c.engine.Enqueue(req)
	if err != nil {
		return
	}

	<-req.Done
	resp = req.RawResp
	return
}
