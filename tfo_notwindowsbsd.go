//go:build !darwin && !freebsd && !windows
// +build !darwin,!freebsd,!windows

package tfo

import (
	"context"
	"net"
	"syscall"
)

func (d *TFODialer) dialTFOContext(ctx context.Context, network, address string) (net.Conn, error) {
	var innerErr error
	d.Dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			innerErr = SetTFODialer(fd)
		})
	}
	c, err := d.Dialer.DialContext(ctx, network, address)
	d.Dialer.Control = nil
	if err != nil {
		return nil, err
	}
	return c, innerErr
}
