//go:build linux

package netpoll

import "golang.org/x/sys/unix"

const (
	ReadEvents      = unix.EPOLLIN | unix.EPOLLERR | unix.EPOLLHUP | unix.EPOLLRDHUP
	WriteEvents     = unix.EPOLLOUT
	ReadWriteEvents = ReadEvents | WriteEvents
)
