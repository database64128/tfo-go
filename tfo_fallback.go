//go:build !darwin && !freebsd && !linux && !windows
// +build !darwin,!freebsd,!linux,!windows

package tfo

import (
	"context"
	"net"
	"syscall"
)

func SetTFOListener(fd uintptr) error {
	return ErrPlatformUnsupported
}

func SetTFODialer(fd uintptr) error {
	return ErrPlatformUnsupported
}

func dialTFO(network string, laddr, raddr *net.TCPAddr, ctrlFn func(string, string, syscall.RawConn) error) (TFOConn, error) {
	d := net.Dialer{
		LocalAddr: laddr,
	}
	c, err := d.DialContext(context.Background(), network, raddr.String())
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: opAddr(laddr), Addr: opAddr(raddr), Err: err}
	}
	return c.(TFOConn), ErrPlatformUnsupported
}
