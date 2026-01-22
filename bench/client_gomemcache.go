//go:build gomemcache

package main

import (
	"github.com/bradfitz/gomemcache/memcache"
)

type Client struct {
	client *memcache.Client
}

func NewClient(cfg *ClientConfig) (*Client, error) {
	cl := memcache.New(cfg.Addr)
	return &Client{client: cl}, nil
}

func (c *Client) Name() string {
	return "gomemcache"
}

func (c *Client) Get(key []byte) ([]byte, error) {
	got, err := c.client.Get(string(key))
	if err != nil {
		return nil, err
	}

	return got.Value, nil
}

func (c *Client) Set(key, val []byte, exp uint32) error {
	return c.client.Set(&memcache.Item{Key: string(key), Value: val, Expiration: int32(exp)})
}

func (c *Client) GetAsync(_ []byte, _ func(v any, e error)) error {
	return nil
}

func (c *Client) SetAsync(_, _ []byte, _ uint32, _ func(v any, e error)) error {
	return nil
}
