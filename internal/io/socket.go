package io

import (
	"github.com/atsegelnyk/memcachex/internal/ring"
	"github.com/atsegelnyk/memcachex/internal/types"
	"golang.org/x/sys/unix"
	"net"
	"time"
)

const (
	defaultTimeout = time.Millisecond * 100
	keepalive      = time.Second * 10

	readBufferDefaultSize  = 1024 * 1024 * 1 // 1mb
	writeBufferDefaultSize = 1024 * 1024 * 1
)

type socket struct {
	fd int

	conn         *net.TCPConn
	inflightRing *ring.SPSC[*types.Req]

	wantPollout bool

	readOffset  int
	rpos        int
	wstart      int
	wend        int
	readBuffer  []byte
	writeBuffer []byte
}

func newSocket(addr string, ringSize int) (*socket, error) {
	tcpCn, err := dial(addr)
	if err != nil {
		return nil, err
	}

	sfd, err := getSocketFd(tcpCn)
	if err != nil {
		return nil, err
	}

	sock := &socket{
		fd:           sfd,
		conn:         tcpCn,
		readBuffer:   make([]byte, readBufferDefaultSize),
		writeBuffer:  make([]byte, writeBufferDefaultSize),
		inflightRing: ring.NewSPSC[*types.Req](ringSize),
	}
	return sock, sock.setOpts()
}

func dial(addr string) (*net.TCPConn, error) {
	dialer := net.Dialer{
		Timeout:   defaultTimeout,
		KeepAlive: keepalive,
	}

	cn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	tcpCn := cn.(*net.TCPConn)
	return tcpCn, nil
}

func getSocketFd(tcpCn *net.TCPConn) (cfd int, err error) {
	rawConn, err := tcpCn.SyscallConn()
	if err != nil {
		return
	}

	err = rawConn.Control(func(fd uintptr) {
		cfd = int(fd)
	})
	return
}

func (s *socket) setOpts() error {
	err := unix.SetsockoptInt(s.fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)
	if err != nil {
		return err
	}

	return unix.SetNonblock(s.fd, true)
}

func (s *socket) read() error {
	for {
		n, err := unix.Read(s.fd, s.readBuffer[s.readOffset:])
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			break
		}
		if err != nil {
			return err
		}

		s.readOffset += n
	}
	return nil
}

func (s *socket) write(b []byte) bool {
	if len(b)+s.wend > len(s.writeBuffer) {
		return false
	}

	copy(s.writeBuffer[s.wend:], b)
	s.wend += len(b)
	s.wantPollout = true
	return true
}

func (s *socket) flush() error {
	for s.wstart < s.wend {
		n, err := unix.Write(s.fd, s.writeBuffer[s.wstart:s.wend])
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			s.wantPollout = true
			return nil
		}
		if err != nil {
			return err
		}
		s.wstart += n
	}

	s.wstart, s.wend = 0, 0
	s.wantPollout = false
	return nil
}

func (s *socket) socketError() error {
	nerr, err := unix.GetsockoptInt(s.fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		return err
	}
	if nerr == 0 {
		return nil
	}
	return unix.Errno(nerr)
}
