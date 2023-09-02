//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"context"
	"net"
)

func (*ListenConfig) listenTFO(ctx context.Context, network, address string) (net.Listener, error) {
	return nil, ErrPlatformUnsupported
}

func (*Dialer) dialTFO(ctx context.Context, network, address string, b []byte) (*net.TCPConn, error) {
	return nil, ErrPlatformUnsupported
}

func dialTCPAddr(network string, laddr, raddr *net.TCPAddr, b []byte) (*net.TCPConn, error) {
	return nil, ErrPlatformUnsupported
}
