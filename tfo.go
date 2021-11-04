package tfo

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

var (
	ErrPlatformUnsupported     = errors.New("tfo-go does not support TCP Fast Open on this platform")
	ErrMismatchedAddressFamily = errors.New("laddr and raddr are not the same address family")
)

type TFOConn interface {
	net.Conn
	io.ReaderFrom
	CloseRead() error
	CloseWrite() error
	SetLinger(sec int) error
	SetNoDelay(noDelay bool) error
	SetKeepAlive(keepalive bool) error
	SetKeepAlivePeriod(d time.Duration) error
}

type TFOListenConfig struct {
	net.ListenConfig

	// DisableTFO controls whether TCP Fast Open is disabled on this instance of TFOListenConfig.
	// TCP Fast Open is enabled by default on TFOListenConfig.
	// Set to true to disable TFO and make TFOListenConfig behave exactly the same as net.ListenConfig.
	DisableTFO bool
}

func (lc *TFOListenConfig) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if !lc.DisableTFO {
			return lc.listenTFO(ctx, network, address) // tfo_darwin.go, tfo_notdarwin.go
		}
	}
	return lc.ListenConfig.Listen(ctx, network, address)
}

func ListenContext(ctx context.Context, network, address string) (net.Listener, error) {
	var lc TFOListenConfig
	return lc.Listen(ctx, network, address)
}

func Listen(network, address string) (net.Listener, error) {
	return ListenContext(context.Background(), network, address)
}

func ListenTCP(network string, laddr *net.TCPAddr) (*net.TCPListener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, &net.OpError{Op: "listen", Net: network, Source: nil, Addr: opAddr(laddr), Err: net.UnknownNetworkError(network)}
	}
	if laddr == nil {
		laddr = &net.TCPAddr{}
	}
	var lc TFOListenConfig
	ln, err := lc.listenTFO(context.Background(), network, laddr.String()) // tfo_darwin.go, tfo_notdarwin.go
	if err != nil && err != ErrPlatformUnsupported {
		return nil, err
	}
	return ln.(*net.TCPListener), err
}

type TFODialer struct {
	net.Dialer

	// DisableTFO controls whether TCP Fast Open is disabled on this instance of TFODialer.
	// TCP Fast Open is enabled by default on TFODialer.
	// Set to true to disable TFO and make TFODialer behave exactly the same as net.Dialer.
	DisableTFO bool
}

func (d *TFODialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if !d.DisableTFO {
			return d.dialTFOContext(ctx, network, address) // tfo_windows_bsd.go, tfo_notwindowsbsd.go
		}
	}
	return d.Dialer.DialContext(ctx, network, address)
}

func (d *TFODialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

func Dial(network, address string) (net.Conn, error) {
	var d TFODialer
	return d.DialContext(context.Background(), network, address)
}

func DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	var d TFODialer
	d.Timeout = timeout
	return d.DialContext(context.Background(), network, address)
}

func DialTCP(network string, laddr, raddr *net.TCPAddr) (TFOConn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return dialTFO(network, laddr, raddr, nil) // tfo_linux.go, tfo_windows.go, tfo_darwin.go, tfo_fallback.go
	default:
		return nil, &net.OpError{Op: "dial", Net: network, Source: opAddr(laddr), Addr: opAddr(raddr), Err: net.UnknownNetworkError(network)}
	}
}

func opAddr(a *net.TCPAddr) net.Addr {
	if a == nil {
		return nil
	}
	return a
}

// wrapSyscallError takes an error and a syscall name. If the error is
// a syscall.Errno, it wraps it in a os.SyscallError using the syscall name.
func wrapSyscallError(name string, err error) error {
	if _, ok := err.(syscall.Errno); ok {
		err = os.NewSyscallError(name, err)
	}
	return err
}
