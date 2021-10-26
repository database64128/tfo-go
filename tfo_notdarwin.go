//go:build !darwin
// +build !darwin

package tfo

import (
	"context"
	"net"
	"syscall"
)

func listen(ctx context.Context, network, address string) (net.Listener, error) {
	var lc net.ListenConfig
	var innerErr error
	switch network {
	case "tcp", "tcp4", "tcp6":
		lc.Control = func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				innerErr = SetTFOListener(fd)
			})
		}
	}
	ln, err := lc.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return ln, innerErr
}
