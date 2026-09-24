//go:build windows && tfogo_checklinkname0

package tfo

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"runtime"
	"unsafe"

	"github.com/database64128/netx-go"
	"golang.org/x/sys/windows"
)

func setIPv6Only(fd windows.Handle, family int, ipv6only bool) error {
	if family == windows.AF_INET6 {
		// Allow both IP versions even if the OS default
		// is otherwise. Note that some operating systems
		// never admit this option.
		return windows.SetsockoptInt(fd, windows.IPPROTO_IPV6, windows.IPV6_V6ONLY, boolint(ipv6only))
	}
	return nil
}

func setNoDelay(fd windows.Handle, noDelay int) error {
	return windows.SetsockoptInt(fd, windows.IPPROTO_TCP, windows.TCP_NODELAY, noDelay)
}

func setUpdateConnectContext(fd windows.Handle) error {
	return windows.Setsockopt(fd, windows.SOL_SOCKET, windows.SO_UPDATE_CONNECT_CONTEXT, nil, 0)
}

func (d *Dialer) dialSingle(ctx context.Context, network string, laddr, raddr netip.AddrPort, b []byte) (*net.TCPConn, error) {
	family, ipv6only := favoriteDialAddrFamily(network, laddr, raddr)

	var lsa windows.Sockaddr
	if laddr.IsValid() {
		var err error
		lsa, err = windowsSockaddrFromAddrPort(laddr, family)
		if err != nil {
			return nil, err
		}
	} else {
		// ConnectEx requires a bound socket.
		switch family {
		case windows.AF_INET:
			lsa = &windows.SockaddrInet4{}
		case windows.AF_INET6:
			lsa = &windows.SockaddrInet6{}
		}
	}

	rsa, err := windowsSockaddrFromAddrPort(raddr, family)
	if err != nil {
		return nil, err
	}

	handle, err := windows.WSASocket(int32(family), windows.SOCK_STREAM, windows.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED|windows.WSA_FLAG_NO_HANDLE_INHERIT)
	if err != nil {
		return nil, os.NewSyscallError("WSASocket", err)
	}

	fd := newFD(handle, family, windows.SOCK_STREAM, network)

	if err = setIPv6Only(handle, family, ipv6only); err != nil {
		fd.Close()
		return nil, os.NewSyscallError("setsockopt(IPV6_V6ONLY)", err)
	}

	if err = setNoDelay(handle, 1); err != nil {
		fd.Close()
		return nil, os.NewSyscallError("setsockopt(TCP_NODELAY)", err)
	}

	if err = setTFODialer(uintptr(handle)); err != nil {
		if !d.Fallback || !errors.Is(err, errors.ErrUnsupported) {
			fd.Close()
			return nil, os.NewSyscallError("setsockopt(TCP_FASTOPEN)", err)
		}
		runtimeDialTFOSupport.StoreNone()
	}

	if d.ControlContext != nil || d.Control != nil {
		ctrlNet := ctrlNetwork(network, family)
		address := raddr.String()
		rawConn := newRawConn(fd)
		var err error
		switch {
		case d.ControlContext != nil:
			err = d.ControlContext(ctx, ctrlNet, address, rawConn)
		case d.Control != nil:
			err = d.Control(ctrlNet, address, rawConn)
		}
		if err != nil {
			fd.Close()
			return nil, err
		}
	}

	if err = windows.Bind(handle, lsa); err != nil {
		fd.Close()
		return nil, wrapSyscallError("bind", err)
	}

	if err = fd.init(); err != nil {
		fd.Close()
		return nil, err
	}

	if err = connWriteFunc(ctx, fd, func(fd *netFD) error {
		n, err := fd.pfd.ConnectEx(rsa, b)
		if err != nil {
			return wrapSyscallError("connectex", err)
		}

		if err = setUpdateConnectContext(handle); err != nil {
			return os.NewSyscallError("setsockopt(SO_UPDATE_CONNECT_CONTEXT)", err)
		}

		lsa, err = windows.Getsockname(handle)
		if err != nil {
			return wrapSyscallError("getsockname", err)
		}
		fd.laddr = tcpAddrFromWindowsSockaddr(lsa)

		rsa, err = windows.Getpeername(handle)
		if err != nil {
			return wrapSyscallError("getpeername", err)
		}
		fd.raddr = tcpAddrFromWindowsSockaddr(rsa)

		if n < len(b) {
			if _, err = fd.Write(b[n:]); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		fd.Close()
		return nil, err
	}

	// This call might get replaced with [runtime.AddCleanup] in the future.
	runtime.SetFinalizer(fd, netFDClose)

	tc := (*net.TCPConn)(unsafe.Pointer(&fd))

	keepAliveCfg := d.KeepAliveConfig
	if !keepAliveCfg.Enable && d.KeepAlive >= 0 {
		keepAliveCfg = net.KeepAliveConfig{
			Enable: true,
			Idle:   d.KeepAlive,
		}
	}
	if keepAliveCfg.Enable {
		_ = tc.SetKeepAliveConfig(keepAliveCfg)
	}

	return tc, nil
}

func windowsSockaddrFromAddrPort(addr netip.AddrPort, family int) (windows.Sockaddr, error) {
	ip := addr.Addr()
	switch family {
	case windows.AF_INET:
		if !ip.Is4() && !ip.Is4In6() {
			return nil, &net.AddrError{Err: "non-IPv4 address", Addr: ip.String()}
		}
		return &windows.SockaddrInet4{
			Port: int(addr.Port()),
			Addr: ip.As4(),
		}, nil
	case windows.AF_INET6:
		if !ip.Is6() {
			return nil, &net.AddrError{Err: "non-IPv6 address", Addr: ip.String()}
		}
		return &windows.SockaddrInet6{
			Port:   int(addr.Port()),
			ZoneId: uint32(netx.ZoneCache.Index(ip.Zone())),
			Addr:   ip.As16(),
		}, nil
	}
	return nil, &net.AddrError{Err: "invalid address family", Addr: ip.String()}
}

func tcpAddrFromWindowsSockaddr(sa windows.Sockaddr) *net.TCPAddr {
	switch sa := sa.(type) {
	case *windows.SockaddrInet4:
		return &net.TCPAddr{IP: sa.Addr[0:], Port: sa.Port}
	case *windows.SockaddrInet6:
		return &net.TCPAddr{IP: sa.Addr[0:], Port: sa.Port, Zone: netx.ZoneCache.Name(int(sa.ZoneId))}
	}
	return nil
}
