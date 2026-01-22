//go:build memcachex

package main

import (
	"github.com/atsegelnyk/memcachex"
	"github.com/atsegelnyk/memcachex/proto"
)

type Client struct {
	client *memcachex.Client
}

func NewClient(cfg *ClientConfig) (*Client, error) {
	cl, err := memcachex.NewClient(
		memcachex.WithAddr(cfg.Addr),
		memcachex.WithNumEventLoops(cfg.NumEventLoops),
		memcachex.WithNumEventLoopSockets(cfg.ConnsPerEventLoop),
		memcachex.WithLockOSThread(cfg.LockOSThread),
	)
	if err != nil {
		return nil, err
	}

	return &Client{client: cl}, nil
}

func (c *Client) Name() string {
	return "memcachex"
}

func (c *Client) Get(key []byte) ([]byte, error) {
	got, err := c.client.Get(key)
	if err != nil {
		return nil, err
	}

	return got.Value, nil
}

func (c *Client) Set(key, val []byte, exp uint32) error {
	return c.client.Set(&proto.Item{Key: key, Value: val, Expiration: exp})
}

func (c *Client) GetAsync(key []byte, cb func(v any, e error)) error {
	return c.client.GetAsync(key, cb)
}

func (c *Client) SetAsync(key, val []byte, exp uint32, cb func(v any, e error)) error {
	return c.client.SetAsync(&proto.Item{Key: key, Value: val, Expiration: exp}, cb)
}
