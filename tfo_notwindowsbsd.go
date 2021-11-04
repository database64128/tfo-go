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
	userCtrlFn := d.Dialer.Control
	d.Dialer.Control = func(network, address string, c syscall.RawConn) error {
		if userCtrlFn != nil {
			if err := userCtrlFn(network, address, c); err != nil {
				return err
			}
		}
		return c.Control(func(fd uintptr) {
			innerErr = SetTFODialer(fd)
		})
	}
	c, err := d.Dialer.DialContext(ctx, network, address)
	d.Dialer.Control = userCtrlFn
	if err != nil {
		return nil, err
	}
	return c, innerErr
}
