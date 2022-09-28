//go:build !darwin

package tfo

import (
	"context"
	"net"
	"syscall"
)

func (lc *ListenConfig) listenTFO(ctx context.Context, network, address string) (net.Listener, error) {
	llc := *lc
	llc.Control = func(network, address string, c syscall.RawConn) (err error) {
		if ctrlFn := lc.Control; ctrlFn != nil {
			if err = ctrlFn(network, address, c); err != nil {
				return
			}
		}
		return c.Control(func(fd uintptr) {
			err = SetTFOListener(fd)
		})
	}
	return llc.ListenConfig.Listen(ctx, network, address)
}
