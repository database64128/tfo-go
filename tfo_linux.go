package tfo

import (
	"context"
	"net"
	"syscall"
	"time"
)

func (d *Dialer) dialTFOContext(ctx context.Context, network, address string, b []byte) (*net.TCPConn, error) {
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
	c := nc.(*net.TCPConn)

	if deadline, ok := ctx.Deadline(); ok {
		c.SetWriteDeadline(deadline)
		defer c.SetWriteDeadline(time.Time{})
	}

	ctxDone := ctx.Done()
	if ctxDone != nil {
		done := make(chan struct{})
		interruptRes := make(chan error)

		defer func() {
			close(done)
			if ctxErr := <-interruptRes; ctxErr != nil && err == nil {
				err = ctxErr
				c.Close()
			}
		}()

		go func() {
			select {
			case <-ctxDone:
				c.SetWriteDeadline(aLongTimeAgo)
				interruptRes <- ctx.Err()
			case <-done:
				interruptRes <- nil
			}
		}()
	}

	if _, err = c.Write(b); err != nil {
		c.Close()
		return nil, err
	}
	return c, err
}

func dialTCPAddr(network string, laddr, raddr *net.TCPAddr, b []byte) (*net.TCPConn, error) {
	d := Dialer{Dialer: net.Dialer{LocalAddr: laddr}}
	return d.dialTFOContext(context.Background(), network, raddr.String(), b)
}
