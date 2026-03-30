package tfo

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const setTFODialerFromSocketSockoptName = "unreachable"

func setTFODialerFromSocket(_ uintptr) error {
	return nil
}

const sendtoImplicitConnectFlag = unix.MSG_FASTOPEN

// doConnectCanFallback returns whether err from [doConnect] indicates lack of TFO support.
func doConnectCanFallback(err error) bool {
	// On Linux, calling sendto() on an unconnected TCP socket with zero or invalid flags
	// returns -EPIPE. This indicates that the MSG_FASTOPEN flag is not recognized by the kernel.
	//
	// -EOPNOTSUPP is returned if the kernel recognizes the flag, but TFO is disabled via sysctl.
	return err == unix.EPIPE || err == unix.EOPNOTSUPP
}

func (a *atomicDialTFOSupport) casLinuxSendto() bool {
	return a.v.CompareAndSwap(uint32(dialTFOSupportDefault), uint32(dialTFOSupportLinuxSendto))
}

func (d *Dialer) dialTFO(ctx context.Context, network, address string, b []byte) (*net.TCPConn, error) {
	fallback := d.Fallback
	if fallback {
		switch runtimeDialTFOSupport.load() {
		case dialTFOSupportNone:
			return d.dialAndWriteTCPConn(ctx, network, address, b)
		case dialTFOSupportLinuxSendto:
			return d.dialTFOFromSocket(ctx, network, address, b)
		}
	}

	var canFallback bool
	ctrlCtxFn := d.ControlContext
	ctrlFn := d.Control
	ld := *d
	// Avoid referencing d in ld.ControlContext to prevent it from being captured by the closure.
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
			if fallback && errors.Is(err, errors.ErrUnsupported) {
				canFallback = true
			}
			return os.NewSyscallError("setsockopt(TCP_FASTOPEN_CONNECT)", err)
		}
		return nil
	}

	nc, err := ld.Dialer.DialContext(ctx, network, address)
	if err != nil {
		if fallback && canFallback {
			runtimeDialTFOSupport.casLinuxSendto()
			return d.dialTFOFromSocket(ctx, network, address, b)
		}
		return nil, err
	}
	tc := nc.(*net.TCPConn)
	if err = netTCPConnWriteBytes(ctx, tc, b); err != nil {
		tc.Close()
		return nil, err
	}
	return tc, nil
}

func (d *Dialer) dialTCP(ctx context.Context, network string, laddr, raddr netip.AddrPort, b []byte) (*net.TCPConn, error) {
	fallback := d.Fallback
	if fallback {
		switch runtimeDialTFOSupport.load() {
		case dialTFOSupportNone:
			return d.dialTCPAndWrite(ctx, network, laddr, raddr, b)
		case dialTFOSupportLinuxSendto:
			return d.dialTCPAddrFromSocket(ctx, network, laddr, raddr, b)
		}
	}

	var canFallback bool
	ctrlCtxFn := d.ControlContext
	ctrlFn := d.Control
	ld := *d
	// Avoid referencing d in ld.ControlContext to prevent it from being captured by the closure.
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
			if fallback && errors.Is(err, errors.ErrUnsupported) {
				canFallback = true
			}
			return os.NewSyscallError("setsockopt(TCP_FASTOPEN_CONNECT)", err)
		}
		return nil
	}

	c, err := ld.Dialer.DialTCP(ctx, network, laddr, raddr)
	if err != nil {
		if fallback && canFallback {
			runtimeDialTFOSupport.casLinuxSendto()
			return d.dialTCPAddrFromSocket(ctx, network, laddr, raddr, b)
		}
		return nil, err
	}
	if err = netTCPConnWriteBytes(ctx, c, b); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
