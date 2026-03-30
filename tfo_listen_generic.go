//go:build freebsd || linux || windows

package tfo

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
)

func (lc *ListenConfig) listenTFO(ctx context.Context, network, address string) (net.Listener, error) {
	// Copy these values to avoid referencing lc in llc.Control.
	ctrlFn := lc.Control
	backlog := lc.Backlog
	fallback := lc.Fallback
	llc := *lc
	llc.Control = func(network, address string, c syscall.RawConn) (err error) {
		if ctrlFn != nil {
			if err = ctrlFn(network, address, c); err != nil {
				return err
			}
		}

		if cerr := c.Control(func(fd uintptr) {
			err = setTFOListenerWithBacklog(fd, backlog)
		}); cerr != nil {
			return cerr
		}

		if err != nil {
			if !fallback || !errors.Is(err, errors.ErrUnsupported) {
				return os.NewSyscallError("setsockopt(TCP_FASTOPEN)", err)
			}
			runtimeListenNoTFO.Store(true)
		}
		return nil
	}
	return llc.ListenConfig.Listen(ctx, network, address)
}
