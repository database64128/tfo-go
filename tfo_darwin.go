package tfo

import (
	"context"
	"net"

	"github.com/database64128/tfo-go/bsd"
	"golang.org/x/sys/unix"
)

func SetTFOListener(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 1)
}

func listen(ctx context.Context, network, address string) (net.Listener, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}

	// darwin requires setting TCP_FASTOPEN after bind() and listen() calls.
	var innerErr error
	switch network {
	case "tcp", "tcp4", "tcp6":
		rawConn, err := ln.(*net.TCPListener).SyscallConn()
		if err != nil {
			ln.Close()
			return nil, err
		}
		err = rawConn.Control(func(fd uintptr) {
			innerErr = SetTFOListener(fd)
		})
		if err != nil {
			ln.Close()
			return nil, err
		}
	}
	return ln, innerErr
}

func SetTFODialer(fd uintptr) error {
	return nil
}

func socket(domain int) (fd int, err error) {
	fd, err = unix.Socket(domain, unix.SOCK_STREAM, unix.IPPROTO_TCP)
	if err != nil {
		return
	}
	unix.CloseOnExec(fd)
	err = unix.SetNonblock(fd, true)
	return
}

func (c *tfoConn) connect(b []byte) (n int, err error) {
	n, err = bsd.Connectx(c.fd, 0, nil, c.rsockaddr, b)
	if err == unix.EINPROGRESS {
		fds := []unix.PollFd{
			{
				Fd:     int32(c.fd),
				Events: unix.POLLWRNORM,
			},
		}
		ret, err := unix.Poll(fds, 0)
		if ret != 1 || err != nil {
			return 0, wrapSyscallError("poll", err)
		}
		return n, nil
	}
	err = wrapSyscallError("connectx", err)
	return
}
