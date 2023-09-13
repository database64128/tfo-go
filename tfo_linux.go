package tfo

import (
	"context"
	"net"
	"syscall"
)

func (d *Dialer) dialTFO(ctx context.Context, network, address string, b []byte) (*net.TCPConn, error) {
	ctrlCtxFn := d.ControlContext
	ctrlFn := d.Control
	ld := *d
	ld.ControlContext = func(ctx context.Context, network, address string, c syscall.RawConn) (err error) {
		switch {
		case ctrlCtxFn != nil:
			if err = ctrlCtxFn(ctx, network, address, c); err != nil {
				return err
			}
		case ctrlFn != nil:
			if err = ctrlFn(network, address, c); err != nil {
				return err
			}
		}

		if cerr := c.Control(func(fd uintptr) {
			err = setTFODialer(fd)
		}); cerr != nil {
			return cerr
		}

		if err != nil {
			return wrapSyscallError("setsockopt(TCP_FASTOPEN_CONNECT)", err)
		}
		return nil
	}

	nc, err := ld.Dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if err = netConnWriteBytes(ctx, nc, b); err != nil {
		nc.Close()
		return nil, err
	}
	return nc.(*net.TCPConn), nil
}

func dialTCPAddr(network string, laddr, raddr *net.TCPAddr, b []byte) (*net.TCPConn, error) {
	d := Dialer{Dialer: net.Dialer{LocalAddr: laddr}}
	return d.dialTFO(context.Background(), network, raddr.String(), b)
}
