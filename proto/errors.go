package proto

import "errors"

var (
	ErrExists    = errors.New("memcachex: exists")
	ErrNotFound  = errors.New("memcachex: not found")
	ErrNotStored = errors.New("memcachex: not stored")
	ErrCacheMiss = errors.New("memcachex: cache miss")

	ErrMemcachedError = errors.New("memcachex: error")
	ErrClientError    = errors.New("memcachex: client error")
	ErrServerError    = errors.New("memcachex: server error")
)
