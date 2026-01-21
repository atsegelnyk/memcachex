package net

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

type Socket struct {
	FD int
	ID int

	conn         *net.TCPConn
	InflightRing *ring.SPSC[*types.Req]

	WantWrite            bool
	PollerWantWriteState bool

	Wstart      int
	Wend        int
	WriteBuffer []byte

	ReadOffset int
	Rpos       int
	ReadBuffer []byte
}

func NewSocket(addr string, ringSize int) (*Socket, error) {
	tcpCn, err := dial(addr)
	if err != nil {
		return nil, err
	}

	sfd, err := getSocketFd(tcpCn)
	if err != nil {
		return nil, err
	}

	sock := &Socket{
		FD:           sfd,
		conn:         tcpCn,
		ReadBuffer:   make([]byte, readBufferDefaultSize),
		WriteBuffer:  make([]byte, writeBufferDefaultSize),
		InflightRing: ring.NewSPSC[*types.Req](ringSize),
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

func (s *Socket) setOpts() error {
	err := unix.SetsockoptInt(s.FD, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)
	if err != nil {
		return err
	}

	return unix.SetNonblock(s.FD, true)
}

func (s *Socket) Read() error {
	for {
		n, err := unix.Read(s.FD, s.ReadBuffer[s.ReadOffset:])
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			break
		}
		if err != nil {
			return err
		}

		s.ReadOffset += n
	}
	return nil
}

func (s *Socket) Write(b []byte) bool {
	if len(b)+s.Wend > len(s.WriteBuffer) {
		return false
	}

	copy(s.WriteBuffer[s.Wend:], b)
	s.Wend += len(b)
	s.WantWrite = true
	return true
}

func (s *Socket) Flush() error {
	for s.Wstart < s.Wend {
		n, err := unix.Write(s.FD, s.WriteBuffer[s.Wstart:s.Wend])
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return nil
		}
		if err != nil {
			return err
		}
		s.Wstart += n
	}

	s.Wstart, s.Wend = 0, 0
	s.WantWrite = false
	return nil
}

func (s *Socket) Close() {
	_ = s.conn.Close()
}
