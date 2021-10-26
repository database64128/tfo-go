package tfo

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
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
}

func ListenContext(ctx context.Context, network, address string) (net.Listener, error) {
	return listen(ctx, network, address)
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
	ln, err := listen(context.Background(), network, laddr.String())
	if err != nil && err != ErrPlatformUnsupported {
		return nil, err
	}
	return ln.(*net.TCPListener), err
}

//FIXME
func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	var innerErr error
	switch network {
	case "tcp", "tcp4", "tcp6":
		d.Control = func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				innerErr = SetTFODialer(fd)
			})
		}
	}
	c, err := d.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return c, innerErr
}

//FIXME
func Dial(network, address string) (net.Conn, error) {
	return DialContext(context.Background(), network, address)
}

func DialTCP(network string, laddr, raddr *net.TCPAddr) (TFOConn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return dialTFO(network, laddr, raddr)
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

type writerOnly struct {
	io.Writer
}

// Fallback implementation of io.ReaderFrom's ReadFrom, when sendfile isn't
// applicable.
func genericReadFrom(w io.Writer, r io.Reader) (n int64, err error) {
	// Use wrapper to hide existing r.ReadFrom from io.Copy.
	return io.Copy(writerOnly{w}, r)
}
