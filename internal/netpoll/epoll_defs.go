//go:build linux

package netpoll

import (
	"github.com/atsegelnyk/memcachex/internal/net"
	"golang.org/x/sys/unix"
	"unsafe"
)

const (
	ReadEvents      = unix.EPOLLIN | unix.EPOLLERR | unix.EPOLLHUP | unix.EPOLLRDHUP
	WriteEvents     = unix.EPOLLOUT
	ReadWriteEvents = ReadEvents | WriteEvents
)

// setPointerDataToEpollEvent takes unsafe.Pointer of socket and stores it
// in epoll_event.data. The socket object is kept alive by eventLoop.
func setPointerDataToEpollEvent(s *net.Socket, ev uint32) *unix.EpollEvent {
	event := &unix.EpollEvent{Events: ev}
	token := uintptr(unsafe.Pointer(s))
	event.Fd = int32(uint32(token))
	event.Pad = int32(uint32(token >> 32))
	return event
}

// getPointerDataFromEpollEvent reconstructs a Go pointer that was previously stored
// in epoll_event.data. The socket object is kept alive by eventLoop.
func getPointerDataFromEpollEvent(ev *unix.EpollEvent) *net.Socket {
	t := uint64(uint32(ev.Fd)) | (uint64(uint32(ev.Pad)) << 32)
	return (*net.Socket)(unsafe.Pointer(uintptr(t)))
}
