//go:build darwin || freebsd

package tfo

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type tfoConn struct {
	mu        sync.Mutex
	fd        int
	f         *os.File
	connected bool
	network   string
	laddr     *net.TCPAddr
	raddr     *net.TCPAddr
	lsockaddr unix.Sockaddr
	rsockaddr unix.Sockaddr
}

func setIPv6Only(fd int, family int, ipv6only int) error {
	if family == unix.AF_INET6 {
		// Allow both IP versions even if the OS default
		// is otherwise. Note that some operating systems
		// never admit this option.
		unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, ipv6only)
	}
	return nil
}

func setNoDelay(fd int, noDelay int) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, noDelay)
}

func setKeepAlive(fd int, keepalive int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_KEEPALIVE, keepalive)
}

func setLinger(fd int, sec int) error {
	var l unix.Linger
	if sec >= 0 {
		l.Onoff = 1
		l.Linger = int32(sec)
	} else {
		l.Onoff = 0
		l.Linger = 0
	}
	return unix.SetsockoptLinger(fd, unix.SOL_SOCKET, unix.SO_LINGER, &l)
}

func ctrlNetwork(network string, family int) string {
	switch network {
	case "tcp4", "tcp6":
		return network
	case "tcp":
		switch family {
		case unix.AF_INET:
			return "tcp4"
		case unix.AF_INET6:
			return "tcp6"
		}
	}
	return "tcp6"
}

func dialTFO(network string, laddr, raddr *net.TCPAddr, ctrlFn func(string, string, syscall.RawConn) error) (Conn, error) {
	var domain int
	var lsockaddr, rsockaddr unix.Sockaddr

	raddr4 := raddr.IP.To4()
	raddrIs4 := raddr4 != nil
	if raddrIs4 {
		domain = unix.AF_INET
		rsockaddr = &unix.SockaddrInet4{
			Port: raddr.Port,
			Addr: *(*[4]byte)(raddr4),
		}
	} else {
		domain = unix.AF_INET6
		rsockaddr = &unix.SockaddrInet6{
			Port: raddr.Port,
			Addr: *(*[16]byte)(raddr.IP),
		}
	}

	if laddr != nil {
		laddr4 := laddr.IP.To4()
		laddrIs4 := laddr4 != nil
		if laddrIs4 != raddrIs4 {
			return nil, ErrMismatchedAddressFamily
		}
		if laddrIs4 {
			lsockaddr = &unix.SockaddrInet4{
				Port: laddr.Port,
				Addr: *(*[4]byte)(laddr4),
			}
		} else {
			lsockaddr = &unix.SockaddrInet6{
				Port: laddr.Port,
				Addr: *(*[16]byte)(laddr.IP),
			}
		}
	} else if raddrIs4 {
		lsockaddr = &unix.SockaddrInet4{}
	} else {
		lsockaddr = &unix.SockaddrInet6{}
	}

	fd, err := socket(domain)
	if err != nil {
		return nil, wrapSyscallError("socket", err)
	}

	var v6only int
	if network == "tcp6" {
		v6only = 1
	}

	if err := setIPv6Only(fd, domain, v6only); err != nil {
		unix.Close(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	if err := setNoDelay(fd, 1); err != nil {
		unix.Close(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	if err := SetTFODialer(uintptr(fd)); err != nil {
		unix.Close(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	f := os.NewFile(uintptr(fd), "")

	if ctrlFn != nil {
		rawConn, err := f.SyscallConn()
		if err != nil {
			unix.Close(fd)
			return nil, err
		}
		if err := ctrlFn(ctrlNetwork(network, domain), raddr.String(), rawConn); err != nil {
			unix.Close(fd)
			return nil, err
		}
	}

	if laddr != nil {
		if err := unix.Bind(fd, lsockaddr); err != nil {
			unix.Close(fd)
			return nil, wrapSyscallError("bind", err)
		}
	}

	return &tfoConn{
		fd:        fd,
		f:         f,
		network:   network,
		laddr:     laddr,
		raddr:     raddr,
		lsockaddr: lsockaddr,
		rsockaddr: rsockaddr,
	}, err
}

func (c *tfoConn) pollWriteReady() error {
	fds := []unix.PollFd{
		{
			Fd:     int32(c.fd),
			Events: unix.POLLWRNORM,
		},
	}

	ret, err := unix.Poll(fds, -1)
	if err != nil {
		return wrapSyscallError("poll", err)
	}
	if ret != 1 {
		return fmt.Errorf("unexpected return value from poll(): %d", ret)
	}
	if fds[0].Revents&unix.POLLWRNORM != unix.POLLWRNORM {
		return fmt.Errorf("unexpected revents from poll(): %d", fds[0].Revents)
	}

	return nil
}

func (c *tfoConn) getLocalAddr() (err error) {
	c.lsockaddr, err = unix.Getsockname(c.fd)
	if err != nil {
		err = wrapSyscallError("getsockname", err)
	}

	switch lsa := c.lsockaddr.(type) {
	case *unix.SockaddrInet4:
		c.laddr = &net.TCPAddr{
			IP:   lsa.Addr[:],
			Port: lsa.Port,
		}
	case *unix.SockaddrInet6: //TODO: convert zone id.
		c.laddr = &net.TCPAddr{
			IP:   lsa.Addr[:],
			Port: lsa.Port,
		}
	}

	return
}

func (c *tfoConn) getSocketError(call string) error {
	nerr, err := unix.GetsockoptInt(c.fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		return wrapSyscallError("getsockopt", err)
	}

	switch err := syscall.Errno(nerr); err {
	case unix.EINPROGRESS, unix.EALREADY, unix.EINTR, unix.EISCONN, syscall.Errno(0):
		return nil
	default:
		return os.NewSyscallError(call, err)
	}
}

func (c *tfoConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	if !c.connected {
		_, err := c.connect(nil)
		if err != nil {
			c.mu.Unlock()
			return 0, &net.OpError{Op: "read", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: err}
		}
		c.connected = true
	}
	c.mu.Unlock()

	n, err := c.f.Read(b)
	if err != nil && err != io.EOF {
		err = &net.OpError{Op: "read", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: err}
	}
	return n, err
}

// ReadFrom utilizes the underlying file's ReadFrom method to minimize copies and allocations.
// This method does not send data in SYN, because application protocols usually write headers
// before calling ReadFrom/WriteTo.
func (c *tfoConn) ReadFrom(r io.Reader) (int64, error) {
	c.mu.Lock()
	if !c.connected {
		_, err := c.connect(nil)
		if err != nil {
			c.mu.Unlock()
			return 0, &net.OpError{Op: "readfrom", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: err}
		}
		c.connected = true
	}
	c.mu.Unlock()
	n, err := c.f.ReadFrom(r)
	if err != nil {
		err = &net.OpError{Op: "readfrom", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: err}
	}
	return n, err
}

func (c *tfoConn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	if !c.connected {
		n, err = c.connect(b)
		if err != nil {
			c.mu.Unlock()
			return 0, &net.OpError{Op: "write", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: err}
		}
		c.connected = true
	}
	c.mu.Unlock()

	if n == len(b) {
		return
	}

	nn, err := c.f.Write(b[n:])
	if err != nil {
		err = &net.OpError{Op: "write", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: err}
	}
	n += nn
	return
}

func (c *tfoConn) Close() error {
	if err := c.f.Close(); err != nil {
		return &net.OpError{Op: "close", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: err}
	}
	return nil
}

func (c *tfoConn) CloseRead() error {
	if err := unix.Shutdown(c.fd, unix.SHUT_RD); err != nil {
		return &net.OpError{Op: "close", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("shutdown", err)}
	}
	return nil
}

func (c *tfoConn) CloseWrite() error {
	if err := unix.Shutdown(c.fd, unix.SHUT_WR); err != nil {
		return &net.OpError{Op: "close", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("shutdown", err)}
	}
	return nil
}

func (c *tfoConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *tfoConn) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *tfoConn) SetDeadline(t time.Time) error {
	if err := c.f.SetDeadline(t); err != nil {
		return &net.OpError{Op: "set", Net: c.network, Source: nil, Addr: c.laddr, Err: err}
	}
	return nil
}

func (c *tfoConn) SetReadDeadline(t time.Time) error {
	if err := c.f.SetReadDeadline(t); err != nil {
		return &net.OpError{Op: "set", Net: c.network, Source: nil, Addr: c.laddr, Err: err}
	}
	return nil
}

func (c *tfoConn) SetWriteDeadline(t time.Time) error {
	if err := c.f.SetWriteDeadline(t); err != nil {
		return &net.OpError{Op: "set", Net: c.network, Source: nil, Addr: c.laddr, Err: err}
	}
	return nil
}

func (c *tfoConn) SetNoDelay(noDelay bool) error {
	var value int
	if noDelay {
		value = 1
	}
	if err := setNoDelay(c.fd, value); err != nil {
		return &net.OpError{Op: "set", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("setsockopt", err)}
	}
	return nil
}

func (c *tfoConn) SetKeepAlive(keepalive bool) error {
	var value int
	if keepalive {
		value = 1
	}
	if err := setKeepAlive(c.fd, value); err != nil {
		return &net.OpError{Op: "set", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("setsockopt", err)}
	}
	return nil
}

func (c *tfoConn) SetKeepAlivePeriod(d time.Duration) error {
	if err := setKeepAlivePeriod(c.fd, d); err != nil {
		return &net.OpError{Op: "set", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("setsockopt", err)}
	}
	return nil
}

func (c *tfoConn) SetLinger(sec int) error {
	if err := setLinger(c.fd, sec); err != nil {
		return &net.OpError{Op: "set", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("setsockopt", err)}
	}
	return nil
}
