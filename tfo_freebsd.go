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
	n, err = unix.SendmsgN(c.fd, b, nil, c.rsockaddr, 0)
	if err == unix.EINPROGRESS { // n == 0
		fds := []unix.PollFd{
			{
				Fd:     int32(c.fd),
				Events: unix.POLLWRNORM,
			},
		}
		ret, err := unix.Poll(fds, 0)
		if err != nil {
			return 0, wrapSyscallError("poll", err)
		}
		if ret != 1 {
			return 0, fmt.Errorf("unexpected return value from poll(): %d", ret)
		}
		if fds[0].Revents&unix.POLLWRNORM != unix.POLLWRNORM {
			return 0, fmt.Errorf("unexpected revents from poll(): %d", fds[0].Revents)
		}
		return c.f.Write(b)
	}
	err = wrapSyscallError("sendmsg", err)
	return
}
