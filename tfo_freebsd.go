package tfo

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

func SetTFOListener(fd uintptr) error {
	return setTFO(fd)
}

func SetTFODialer(fd uintptr) error {
	return setTFO(fd)
}

func setTFO(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 1)
}

func setKeepAlivePeriod(fd int, d time.Duration) error {
	// The kernel expects seconds so round to next highest second.
	secs := int(roundDurationUp(d, time.Second))
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, secs); err != nil {
		return err
	}
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPIDLE, secs)
}

func socket(domain int) (int, error) {
	return unix.Socket(domain, unix.SOCK_STREAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.IPPROTO_TCP)
}

func (c *tfoConn) connect(b []byte) (n int, err error) {
	rawConn, err := c.f.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("failed to get syscall.RawConn: %w", err)
	}

	var done bool
	perr := rawConn.Write(func(fd uintptr) bool {
		if done {
			return true
		}

		n, err = unix.SendmsgN(c.fd, b, nil, c.rsockaddr, 0)
		switch err {
		case unix.EINPROGRESS:
			done = true
			err = nil
			return false
		case unix.EAGAIN:
			return false
		default:
			return true
		}
	})

	if err != nil {
		return 0, wrapSyscallError("sendmsg", err)
	}

	if perr != nil {
		return 0, perr
	}

	err = c.getSocketError("sendmsg")
	if err != nil {
		return
	}

	err = c.getLocalAddr()
	return
}
