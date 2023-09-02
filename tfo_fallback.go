//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"context"
	"net"
)

func (d *Dialer) dialTFOContext(ctx context.Context, network, address string, b []byte) (*net.TCPConn, error) {
	return nil, ErrPlatformUnsupported
}

func dialTCPAddr(network string, laddr, raddr *net.TCPAddr, b []byte) (*net.TCPConn, error) {
	return nil, ErrPlatformUnsupported
}
