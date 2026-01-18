package proto

import (
	"github.com/pkg/errors"
)

var (
	ErrExists    = errors.New("memcached: exists")
	ErrNotFound  = errors.New("memcached: not found")
	ErrNotStored = errors.New("memcached: not stored")
	ErrCacheMiss = errors.New("memcached: cache miss")

	ErrMemcachedError = errors.New("memcached: error")
	ErrClientError    = errors.New("memcached: client error")
	ErrServerError    = errors.New("memcached: server error")
)

var (
	ErrBusy              = errors.New("engine busy")
	ErrNoActiveEventLoop = errors.New("no active event loop")
)
