//go:build !darwin
// +build !darwin

package tfo

import (
	"context"
	"net"
	"syscall"
)

func (lc *TFOListenConfig) listenTFO(ctx context.Context, network, address string) (net.Listener, error) {
	var innerErr error
	userCtrlFn := lc.ListenConfig.Control
	lc.ListenConfig.Control = func(network, address string, c syscall.RawConn) error {
		if userCtrlFn != nil {
			if err := userCtrlFn(network, address, c); err != nil {
				return err
			}
		}
		return c.Control(func(fd uintptr) {
			innerErr = SetTFOListener(fd)
		})
	}
	ln, err := lc.ListenConfig.Listen(ctx, network, address)
	lc.ListenConfig.Control = userCtrlFn
	if err != nil {
		return nil, err
	}
	return ln, innerErr
}
