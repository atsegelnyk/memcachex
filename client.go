package memcachex

import (
	"github.com/atsegelnyk/memcachex/internal/io"
	"github.com/atsegelnyk/memcachex/internal/pool"
	"github.com/atsegelnyk/memcachex/internal/proto/ascii"
	"github.com/atsegelnyk/memcachex/internal/types"
	"github.com/atsegelnyk/memcachex/proto"
	"runtime"
	"time"
)

const (
	defaultLockOSThread        = false
	defaultNumEventLoops       = 1
	defaultNumEventLoopSockets = 2
	defaultRingSize            = 8192
	defaultAddr                = "localhost:11211"
	defaultNumEnqueueRetries   = 1
)

type Client struct {
	numEnqueueRetries int

	opts   *ClientOptions
	engine *io.Engine
}

type ClientOptions struct {
	Addr string

	LockOSThread        bool
	NumEventLoops       int
	NumEventLoopSockets int
	RingSize            int

	NumEnqueueRetries int
}

type ClientOption func(*Client)

func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		opts: &ClientOptions{
			Addr:                defaultAddr,
			NumEventLoops:       defaultNumEventLoops,
			LockOSThread:        defaultLockOSThread,
			NumEventLoopSockets: defaultNumEventLoopSockets,
			RingSize:            defaultRingSize,
			NumEnqueueRetries:   defaultNumEnqueueRetries,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	c.numEnqueueRetries = c.opts.NumEnqueueRetries

	engine, err := io.NewEngine(
		c.opts.Addr,
		c.opts.NumEventLoops,
		c.opts.NumEventLoopSockets,
		c.opts.RingSize,
		c.opts.LockOSThread,
	)
	if err != nil {
		return nil, err
	}

	c.engine = engine
	return c, nil
}

// WithNumEventLoops specifies number of eventLoops, that will serve.
//
// Defaults to 1.
//
// Each eventLoop has its own sockets and request buffers.
func WithNumEventLoops(num int) ClientOption {
	return func(c *Client) {
		c.opts.NumEventLoops = num
	}
}

// WithNumEventLoopSockets specifies number of sockets, that will be served per eventLoop.
//
// Defaults to 2
func WithNumEventLoopSockets(num int) ClientOption {
	return func(c *Client) {
		c.opts.NumEventLoopSockets = num
	}
}

// WithLockOSThread sets runtime.LockOSThread for the eventLoop goroutine.
//
// Defaults to false
func WithLockOSThread(lock bool) ClientOption {
	return func(c *Client) {
		c.opts.LockOSThread = lock
	}
}

// WithAddr sets memcached hots address.
//
// Defaults to localhost:11211
func WithAddr(addr string) ClientOption {
	return func(c *Client) {
		c.opts.Addr = addr
	}
}

// WithRingSize sets request ring buffer size for eventLoop.
//
// Defaults to 8192
func WithRingSize(size int) ClientOption {
	return func(c *Client) {
		c.opts.RingSize = size
	}
}

// WithOptions sets custom ClientOptions.
func WithOptions(opts *ClientOptions) ClientOption {
	return func(c *Client) {
		c.opts = opts
	}
}

// WithNumEnqueueRetries sets number of enqueue retries that will be
// made by the client if engine is busy at the moment.
//
// Defaults to 1
func WithNumEnqueueRetries(numRetries int) ClientOption {
	return func(c *Client) {
		c.opts.NumEnqueueRetries = numRetries
	}
}

// Get fetches a value for the given key using the Memcached "get" command.
//
// It encodes a request, performs a synchronous round trip to the server,
// and decodes the response into a *proto.Value.
//
// Errors:
//   - Returns an error if round-trip fails.
//   - Returns an error if the request fails
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

// GetAsync enqueues a Memcached "get" request for asynchronous execution.
//
// The call returns immediately after the request is enqueued. Once the response
// is received and decoded, callerCb is invoked if non nil.
//
// The callback receives:
//   - a decoded *proto.Value (as `any`) and a nil error on success
//   - a non-nil error if the request fails
//
// Returns:
//   - nil if the request was successfully enqueued
//   - an error if the request could not be enqueued
func (c *Client) GetAsync(key []byte, callerCb proto.CallerCallback) error {
	raw := ascii.EncodeGet(key)

	req := pool.ReqPool.Get().(*types.Req)
	req.Raw = raw
	req.CmdCallback = getCallback
	req.CallerCallback = callerCb
	req.NonBlocking = true
	req.TS = time.Now().Unix()

	return c.enqueueRequest(req)
}

func getCallback(callerCb proto.CallerCallback, r []byte, e error) {
	defer pool.BufferPool.Put(r)
	if callerCb == nil {
		return
	}

	if e != nil {
		callerCb(nil, e)
		return
	}

	resp, err := ascii.DecodeGetResponse(r)
	callerCb(resp, err)
}

// GetMulti fetches multiple values using a single Memcached "get" request.
//
// It encodes a multi-key request, performs a synchronous round trip to the server,
// and decodes the response into a slice of *proto.Value.
//
// Returns:
//   - ([]*proto.Value, nil) on success (may contain multiple items).
//   - (nil, error) if the request fails.
func (c *Client) GetMulti(keys ...[]byte) ([]*proto.Value, error) {
	raw := ascii.EncodeGetMulti(keys)

	resp, err := c.roundTrip(raw)
	if err != nil {
		return nil, err
	}

	getMultiResp, err := ascii.DecodeGetMultiResponse(resp)
	pool.BufferPool.Put(resp)

	return getMultiResp, err
}

// GetMultiAsync enqueues a multi-key Memcached "get" request for asynchronous execution.
//
// The call returns immediately after the request is enqueued. Once the response is
// received and decoded, callerCb is invoked if non nil.
//
// The callback receives:
//   - a decoded []*proto.Value (as `any`) and a nil error on success
//   - a non-nil error if the request fails or the response cannot be decoded
//
// Returns:
//   - nil if the request was successfully enqueued
//   - an error if the request could not be enqueued
func (c *Client) GetMultiAsync(callerCb proto.CallerCallback, keys ...[]byte) error {
	raw := ascii.EncodeGetMulti(keys)

	req := pool.ReqPool.Get().(*types.Req)
	req.Raw = raw
	req.CmdCallback = getMuliCallback
	req.CallerCallback = callerCb
	req.NonBlocking = true
	req.TS = time.Now().Unix()

	return c.enqueueRequest(req)
}

func getMuliCallback(callerCb proto.CallerCallback, r []byte, e error) {
	defer pool.BufferPool.Put(r)
	if callerCb == nil {
		return
	}

	if e != nil {
		callerCb(nil, e)
		return
	}

	resp, err := ascii.DecodeGetMultiResponse(r)
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
	req.NonBlocking = true
	req.TS = time.Now().Unix()

	return c.enqueueRequest(req)
}

func setCallback(callerCb proto.CallerCallback, r []byte, e error) {
	defer pool.BufferPool.Put(r)
	if callerCb == nil {
		return
	}

	if e != nil {
		callerCb(nil, e)
		return
	}

	err := ascii.DecodeSetResponse(r)
	callerCb(nil, err)
}

func (c *Client) Delete(key []byte) error {
	raw := ascii.EncodeDelete(key)
	resp, err := c.roundTrip(raw)
	if err != nil {
		return err
	}

	err = ascii.DecodeDeleteResponse(resp)
	pool.BufferPool.Put(resp)

	return err
}

func (c *Client) DeleteAsync(key []byte, callerCb proto.CallerCallback) error {
	raw := ascii.EncodeDelete(key)

	req := pool.ReqPool.Get().(*types.Req)
	req.Raw = raw
	req.CmdCallback = deleteCallback
	req.CallerCallback = callerCb
	req.NonBlocking = true
	req.TS = time.Now().Unix()

	return c.enqueueRequest(req)
}

func deleteCallback(callerCb proto.CallerCallback, r []byte, e error) {
	defer pool.BufferPool.Put(r)
	if callerCb == nil {
		return
	}

	if e != nil {
		callerCb(nil, e)
		return
	}

	err := ascii.DecodeDeleteResponse(r)
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
	req.NonBlocking = true
	req.TS = time.Now().Unix()

	return c.enqueueRequest(req)
}

func versionCallback(callerCb proto.CallerCallback, r []byte, e error) {
	defer pool.BufferPool.Put(r)
	if callerCb == nil {
		return
	}

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
	req.NonBlocking = false

	err = c.enqueueRequest(req)
	if err != nil {
		return
	}

	<-req.Done
	resp = req.RawResp
	return
}

func (c *Client) enqueueRequest(req *types.Req) error {
	attempts := c.numEnqueueRetries + 1

	for i := 0; i < attempts; i++ {
		err := c.engine.Enqueue(req)
		if err == nil {
			return nil
		}

		if err != proto.ErrBusy {
			return err
		}

		if i+1 < attempts {
			runtime.Gosched()
		}
	}

	return proto.ErrBusy
}
