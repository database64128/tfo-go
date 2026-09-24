//go:build darwin || freebsd || linux

package tfo

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"syscall"

	"github.com/database64128/netx-go"
	"golang.org/x/sys/unix"
)

func setIPv6Only(fd int, family int, ipv6only bool) error {
	if family == unix.AF_INET6 {
		// Allow both IP versions even if the OS default
		// is otherwise. Note that some operating systems
		// never admit this option.
		return unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, boolint(ipv6only))
	}
	return nil
}

func setNoDelay(fd int, noDelay int) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, noDelay)
}

func (d *Dialer) dialSingle(ctx context.Context, network string, laddr, raddr netip.AddrPort, b []byte) (*net.TCPConn, error) {
	family, ipv6only := favoriteDialAddrFamily(network, laddr, raddr)

	fd, err := d.socket(family)
	if err != nil {
		return nil, wrapSyscallError("socket", err)
	}

	if err = d.setIPv6Only(fd, family, ipv6only); err != nil {
		unix.Close(fd)
		return nil, os.NewSyscallError("setsockopt(IPV6_V6ONLY)", err)
	}

	if err = setNoDelay(fd, 1); err != nil {
		unix.Close(fd)
		return nil, os.NewSyscallError("setsockopt(TCP_NODELAY)", err)
	}

	if err = setTFODialerFromSocket(uintptr(fd)); err != nil {
		if !d.Fallback || !errors.Is(err, errors.ErrUnsupported) {
			unix.Close(fd)
			return nil, os.NewSyscallError("setsockopt("+setTFODialerFromSocketSockoptName+")", err)
		}
		runtimeDialTFOSupport.storeNone()
	}

	f := os.NewFile(uintptr(fd), "")
	defer f.Close()

	rawConn, err := f.SyscallConn()
	if err != nil {
		return nil, err
	}

	if d.ControlContext != nil || d.Control != nil {
		ctrlNet := ctrlNetwork(network, family)
		address := raddr.String()
		switch {
		case d.ControlContext != nil:
			err = d.ControlContext(ctx, ctrlNet, address, rawConn)
		case d.Control != nil:
			err = d.Control(ctrlNet, address, rawConn)
		}
		if err != nil {
			return nil, err
		}
	}

	if laddr.IsValid() {
		lsa, err := unixSockaddrFromAddrPort(laddr, family)
		if err != nil {
			return nil, err
		}

		if cErr := rawConn.Control(func(fd uintptr) {
			err = unix.Bind(int(fd), lsa)
		}); cErr != nil {
			return nil, cErr
		}
		if err != nil {
			return nil, wrapSyscallError("bind", err)
		}
	}

	rsa, err := unixSockaddrFromAddrPort(raddr, family)
	if err != nil {
		return nil, err
	}

	var (
		n           int
		canFallback bool
	)

	if err = connWriteFunc(ctx, f, func(f *os.File) (err error) {
		n, canFallback, err = connect(rawConn, rsa, b)
		return err
	}); err != nil {
		if d.Fallback && canFallback {
			runtimeDialTFOSupport.storeNone()
			return d.dialTCPAndWrite(ctx, network, laddr, raddr, b)
		}
		return nil, err
	}

	c, err := net.FileConn(f)
	if err != nil {
		return nil, err
	}
	tc := c.(*net.TCPConn)

	// [net.FileConn] enables TCP keep-alive with default settings.
	switch {
	case d.KeepAliveConfig.Enable:
		_ = tc.SetKeepAliveConfig(d.KeepAliveConfig)
	case d.KeepAlive < 0:
		_ = tc.SetKeepAlive(false)
	case d.KeepAlive > 0:
		_ = tc.SetKeepAlivePeriod(d.KeepAlive)
	}

	if n < len(b) {
		if err = netTCPConnWriteBytes(ctx, tc, b[n:]); err != nil {
			tc.Close()
			return nil, err
		}
	}

	return tc, err
}

func unixSockaddrFromAddrPort(addr netip.AddrPort, family int) (unix.Sockaddr, error) {
	ip := addr.Addr()
	switch family {
	case unix.AF_INET:
		if !ip.Is4() && !ip.Is4In6() {
			return nil, &net.AddrError{Err: "non-IPv4 address", Addr: ip.String()}
		}
		return &unix.SockaddrInet4{
			Port: int(addr.Port()),
			Addr: ip.As4(),
		}, nil
	case unix.AF_INET6:
		if !ip.Is6() {
			return nil, &net.AddrError{Err: "non-IPv6 address", Addr: ip.String()}
		}
		return &unix.SockaddrInet6{
			Port:   int(addr.Port()),
			ZoneId: uint32(netx.ZoneCache.Index(ip.Zone())),
			Addr:   ip.As16(),
		}, nil
	}
	return nil, &net.AddrError{Err: "invalid address family", Addr: ip.String()}
}

func connect(rawConn syscall.RawConn, rsa unix.Sockaddr, b []byte) (n int, canFallback bool, err error) {
	var done bool

	if perr := rawConn.Write(func(fd uintptr) bool {
		if done {
			return true
		}

		n, err = doConnect(fd, rsa, b)
		if err == unix.EINPROGRESS {
			done = true
			err = nil
			return false
		}
		return true
	}); perr != nil {
		return 0, false, perr
	}

	if err != nil {
		return 0, doConnectCanFallback(err), wrapSyscallError(connectSyscallName, err)
	}

	if perr := rawConn.Control(func(fd uintptr) {
		err = getSocketError(int(fd), connectSyscallName)
	}); perr != nil {
		return 0, false, perr
	}

	return
}

func getSocketError(fd int, call string) error {
	nerr, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		return os.NewSyscallError("getsockopt", err)
	}
	if nerr != 0 {
		return os.NewSyscallError(call, syscall.Errno(nerr))
	}
	return nil
}
