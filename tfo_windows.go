package tfo

import (
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/database64128/tfo-go/winsock2"
	"golang.org/x/sys/windows"
)

const TCP_FASTOPEN = 15

func SetTFOListener(fd uintptr) error {
	return setTFO(windows.Handle(fd))
}

func SetTFODialer(fd uintptr) error {
	return setTFO(windows.Handle(fd))
}

func setTFO(fd windows.Handle) error {
	return windows.SetsockoptInt(fd, windows.IPPROTO_TCP, TCP_FASTOPEN, 1)
}

func setIPv6Only(fd windows.Handle, family int, ipv6only int) error {
	if family == windows.AF_INET6 {
		// Allow both IP versions even if the OS default
		// is otherwise. Note that some operating systems
		// never admit this option.
		windows.SetsockoptInt(fd, windows.IPPROTO_IPV6, windows.IPV6_V6ONLY, ipv6only)
	}
	return nil
}

func setNoDelay(fd windows.Handle, noDelay int) error {
	return windows.SetsockoptInt(fd, windows.IPPROTO_TCP, windows.TCP_NODELAY, noDelay)
}

func setKeepAlive(fd windows.Handle, keepalive int) error {
	return windows.SetsockoptInt(fd, windows.SOL_SOCKET, windows.SO_KEEPALIVE, keepalive)
}

func setKeepAlivePeriod(fd windows.Handle, d time.Duration) error {
	// The kernel expects milliseconds so round to next highest
	// millisecond.
	msecs := uint32(roundDurationUp(d, time.Millisecond))
	ka := windows.TCPKeepalive{
		OnOff:    1,
		Time:     msecs,
		Interval: msecs,
	}
	ret := uint32(0)
	size := uint32(unsafe.Sizeof(ka))
	return windows.WSAIoctl(fd, windows.SIO_KEEPALIVE_VALS, (*byte)(unsafe.Pointer(&ka)), size, nil, 0, &ret, nil, 0)
}

func setLinger(fd windows.Handle, sec int) error {
	var l windows.Linger
	if sec >= 0 {
		l.Onoff = 1
		l.Linger = int32(sec)
	} else {
		l.Onoff = 0
		l.Linger = 0
	}
	return windows.SetsockoptLinger(fd, windows.SOL_SOCKET, windows.SO_LINGER, &l)
}

func setUpdateConnectContext(fd windows.Handle) error {
	return windows.Setsockopt(fd, windows.SOL_SOCKET, windows.SO_UPDATE_CONNECT_CONTEXT, nil, 0)
}

type tfoConn struct {
	mu            sync.Mutex
	fd            windows.Handle
	connected     bool
	network       string
	laddr         *net.TCPAddr
	raddr         *net.TCPAddr
	lsockaddr     windows.Sockaddr
	rsockaddr     windows.Sockaddr
	readDeadline  time.Time
	writeDeadline time.Time
}

// rawConn implements syscall.RawConn.
type rawConn struct {
	fd windows.Handle
}

func (c *rawConn) Control(f func(uintptr)) error {
	f(uintptr(c.fd))
	return nil
}

func (c *rawConn) Read(f func(uintptr) bool) error {
	for {
		if f(uintptr(c.fd)) {
			return nil
		}

		_, err := winsock2.Recv(c.fd, nil, 0)
		if err != nil && err != windows.WSAEMSGSIZE {
			return err
		}
	}
}

func (c *rawConn) Write(f func(uintptr) bool) error {
	if f(uintptr(c.fd)) {
		return nil
	}
	return syscall.EWINDOWS
}

func ctrlNetwork(network string, family int) string {
	switch network {
	case "tcp4", "tcp6":
		return network
	case "tcp":
		switch family {
		case windows.AF_INET:
			return "tcp4"
		case windows.AF_INET6:
			return "tcp6"
		}
	}
	return "tcp6"
}

func dialTFO(network string, laddr, raddr *net.TCPAddr, ctrlFn func(string, string, syscall.RawConn) error) (TFOConn, error) {
	var domain int
	var lsockaddr, rsockaddr windows.Sockaddr

	raddr4 := raddr.IP.To4()
	raddrIs4 := raddr4 != nil
	if raddrIs4 {
		domain = windows.AF_INET
		rsockaddr = &windows.SockaddrInet4{
			Port: raddr.Port,
			Addr: *(*[4]byte)(raddr4),
		}
	} else {
		domain = windows.AF_INET6
		rsockaddr = &windows.SockaddrInet6{
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
			lsockaddr = &windows.SockaddrInet4{
				Port: laddr.Port,
				Addr: *(*[4]byte)(laddr4),
			}
		} else {
			lsockaddr = &windows.SockaddrInet6{
				Port: laddr.Port,
				Addr: *(*[16]byte)(laddr.IP),
			}
		}
	} else if raddrIs4 {
		lsockaddr = &windows.SockaddrInet4{}
	} else {
		lsockaddr = &windows.SockaddrInet6{}
	}

	fd, err := windows.Socket(domain, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, wrapSyscallError("socket", err)
	}

	var v6only int
	if network == "tcp6" {
		v6only = 1
	}

	if err := setIPv6Only(fd, domain, v6only); err != nil {
		windows.Closesocket(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	if err := setNoDelay(fd, 1); err != nil {
		windows.Closesocket(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	if err := setTFO(fd); err != nil {
		windows.Closesocket(fd)
		return nil, wrapSyscallError("setsockopt", err)
	}

	if ctrlFn != nil {
		if err := ctrlFn(ctrlNetwork(network, domain), raddr.String(), &rawConn{fd: fd}); err != nil {
			windows.Closesocket(fd)
			return nil, err
		}
	}

	if err := windows.Bind(fd, lsockaddr); err != nil {
		windows.Closesocket(fd)
		return nil, wrapSyscallError("bind", err)
	}

	return &tfoConn{
		fd:        fd,
		network:   network,
		laddr:     laddr,
		raddr:     raddr,
		lsockaddr: lsockaddr,
		rsockaddr: rsockaddr,
	}, err
}

// connect calls ConnectEx with an optional first data to send in SYN.
// This method does not check the connected variable.
// Lock the mutex and only call this method if connected is false.
// After the call, if the returned n is greater than 0 or error is nil, set connected to true.
func (c *tfoConn) connect(b []byte) (n int, err error) {
	var bytesSent uint32
	var flags uint32
	var overlapped windows.Overlapped
	var sendBuf *byte
	if len(b) > 0 {
		sendBuf = &b[0]
	}

	efd, err := winsock2.WSACreateEvent()
	if err != nil {
		err = wrapSyscallError("WSACreateEvent", err)
		return
	}
	overlapped.HEvent = efd

	err = windows.ConnectEx(c.fd, c.rsockaddr, sendBuf, uint32(len(b)), &bytesSent, &overlapped)
	if err == windows.ERROR_IO_PENDING {
		err = windows.WSAGetOverlappedResult(c.fd, &overlapped, &bytesSent, true, &flags)
	}
	if err != nil {
		err = wrapSyscallError("ConnectEx", err)
		return
	}
	n = int(bytesSent)
	err = setUpdateConnectContext(c.fd)
	return
}

func (c *tfoConn) Read(b []byte) (int, error) {
	if !c.readDeadline.IsZero() && c.readDeadline.Before(time.Now()) {
		return 0, &net.OpError{Op: "read", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: os.ErrDeadlineExceeded}
	}

	c.mu.Lock()
	if !c.connected {
		_, err := c.connect(nil)
		if err != nil {
			c.mu.Unlock()
			return 0, err
		}
		c.connected = true
	}
	c.mu.Unlock()

	n, err := winsock2.Recv(c.fd, b, 0)
	if n == 0 && err == nil {
		return 0, io.EOF
	}
	return int(n), wrapSyscallError("recv", err)
}

// ReadFrom utilizes the underlying file's ReadFrom method to minimize copies and allocations.
// This method does not send data in SYN, because application protocols usually write headers
// before calling ReadFrom/WriteTo.
func (c *tfoConn) ReadFrom(r io.Reader) (int64, error) {
	if !c.writeDeadline.IsZero() && c.writeDeadline.Before(time.Now()) {
		return 0, &net.OpError{Op: "readfrom", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: os.ErrDeadlineExceeded}
	}

	c.mu.Lock()
	if !c.connected {
		_, err := c.connect(nil)
		if err != nil {
			c.mu.Unlock()
			return 0, err
		}
		c.connected = true
	}
	c.mu.Unlock()

	if n, handled, err := c.sendFile(r); handled {
		return n, wrapSyscallError("transmitfile", err)
	}
	return genericReadFrom(c, r)
}

func (c *tfoConn) sendFile(r io.Reader) (written int64, handled bool, err error) {
	var n int64 = 0 // by default, copy until EOF.
	var overlapped windows.Overlapped

	lr, ok := r.(*io.LimitedReader)
	if ok {
		n, r = lr.N, lr.R
		if n <= 0 {
			return 0, true, nil
		}
	}

	f, ok := r.(*os.File)
	if !ok {
		return 0, false, nil
	}
	fh := windows.Handle(f.Fd())

	// TODO(brainman): skip calling windows.Seek if OS allows it
	curpos, err := windows.Seek(fh, 0, io.SeekCurrent)
	if err != nil {
		return 0, false, err
	}

	if n <= 0 { // We don't know the size of the file so infer it.
		// Find the number of bytes offset from curpos until the end of the file.
		n, err = windows.Seek(fh, -curpos, io.SeekEnd)
		if err != nil {
			return
		}
		// Now seek back to the original position.
		if _, err = windows.Seek(fh, curpos, io.SeekStart); err != nil {
			return
		}
	}

	// TransmitFile can be invoked in one call with at most
	// 2,147,483,646 bytes: the maximum value for a 32-bit integer minus 1.
	// See https://docs.microsoft.com/en-us/windows/win32/api/mswsock/nf-mswsock-transmitfile
	const maxChunkSizePerCall = int64(0x7fffffff - 1)

	for n > 0 {
		var bytesSent uint32
		var flags uint32
		chunkSize := maxChunkSizePerCall
		if chunkSize > n {
			chunkSize = n
		}

		overlapped.Offset = uint32(curpos)
		overlapped.OffsetHigh = uint32(curpos >> 32)

		err = windows.TransmitFile(c.fd, fh, uint32(chunkSize), 0, &overlapped, nil, windows.TF_WRITE_BEHIND)
		if err == windows.ERROR_IO_PENDING {
			err = windows.WSAGetOverlappedResult(c.fd, &overlapped, &bytesSent, true, &flags)
		}
		if err != nil {
			return written, true, err
		}

		curpos += int64(bytesSent)

		// Some versions of Windows (Windows 10 1803) do not set
		// file position after TransmitFile completes.
		// So just use Seek to set file position.
		if _, err = windows.Seek(fh, curpos, io.SeekStart); err != nil {
			return written, true, err
		}

		n -= int64(bytesSent)
		written += int64(bytesSent)
	}

	// If any byte was copied, regardless of any error
	// encountered mid-way, handled must be set to true.
	handled = written > 0

	return
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

func (c *tfoConn) Write(b []byte) (int, error) {
	if !c.writeDeadline.IsZero() && c.writeDeadline.Before(time.Now()) {
		return 0, &net.OpError{Op: "write", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: os.ErrDeadlineExceeded}
	}

	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		n, err := winsock2.Send(c.fd, b, 0)
		return int(n), wrapSyscallError("send", err)
	}

	n, err := c.connect(b)
	if n > 0 || err == nil { // setUpdateConnectContext could return error.
		c.connected = true
	}
	c.mu.Unlock()
	return n, err
}

func (c *tfoConn) Close() error {
	windows.Shutdown(c.fd, windows.SHUT_RDWR)
	windows.Closesocket(c.fd)
	return nil
}

func (c *tfoConn) CloseRead() error {
	if err := windows.Shutdown(c.fd, windows.SHUT_RD); err != nil {
		return &net.OpError{Op: "close", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("shutdown", err)}
	}
	return nil
}

func (c *tfoConn) CloseWrite() error {
	if err := windows.Shutdown(c.fd, windows.SHUT_WR); err != nil {
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
	c.SetReadDeadline(t)
	c.SetWriteDeadline(t)
	return nil
}

func (c *tfoConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *tfoConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
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
		return &net.OpError{Op: "set", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("WSAIoctl", err)}
	}
	return nil
}

func (c *tfoConn) SetLinger(sec int) error {
	if err := setLinger(c.fd, sec); err != nil {
		return &net.OpError{Op: "set", Net: c.network, Source: c.laddr, Addr: c.raddr, Err: wrapSyscallError("setsockopt", err)}
	}
	return nil
}
