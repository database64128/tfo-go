package tfo

import (
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

func socket(domain int) (int, error) {
	return unix.Socket(domain, unix.SOCK_STREAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.IPPROTO_TCP)
}

func (c *tfoConn) connect(b []byte) (n int, err error) {
	n, err = unix.SendmsgN(c.fd, b, nil, c.rsockaddr, 0)
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
	err = wrapSyscallError("sendmsg", err)
	return
}
