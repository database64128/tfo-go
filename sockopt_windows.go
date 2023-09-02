package tfo

import "golang.org/x/sys/windows"

const TCP_FASTOPEN = 15

func setTFO(fd, value int) error {
	return windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_TCP, TCP_FASTOPEN, value)
}
