package tfo

import (
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
	n, err = unix.SendmsgN(c.fd, b, nil, c.rsockaddr, 0)
	if err != nil && err != unix.EINPROGRESS {
		err = wrapSyscallError("sendmsg", err)
		return
	}
	if err == unix.EINPROGRESS { // n == 0
		err = c.pollWriteReady()
		if err != nil {
			return
		}
	}

	err = c.getSocketError("sendmsg")
	if err != nil {
		return
	}

	err = c.getLocalAddr()
	if err != nil {
		return
	}

	if n == 0 && len(b) > 0 {
		return c.f.Write(b)
	}

	return
}
