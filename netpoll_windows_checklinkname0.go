//go:build windows && tfogo_checklinkname0

package tfo

import (
	"net"
	"time"
	_ "unsafe"

	"golang.org/x/sys/windows"
)

// operation contains superset of data necessary to perform all async IO.
//
// Copied from src/internal/poll/fd_windows.go
type operation struct {
	// Used by IOCP interface, it must be first field
	// of the struct, as our code relies on it.
	o windows.Overlapped

	// fields used by runtime.netpoll
	runtimeCtx uintptr
	mode       int32

	// fields used only by net package
	fd     *pFD
	buf    windows.WSABuf
	msg    windows.WSAMsg
	sa     windows.Sockaddr
	rsa    *windows.RawSockaddrAny
	rsan   int32
	handle windows.Handle
	flags  uint32
	qty    uint32
	bufs   []windows.WSABuf
}

//go:linkname execIO internal/poll.execIO
func execIO(o *operation, submit func(o *operation) error) (int, error)

func (fd *pFD) ConnectEx(ra windows.Sockaddr, b []byte) (n int, err error) {
	n, err = execIO(&fd.wop, func(o *operation) error {
		return windows.ConnectEx(o.fd.Sysfd, ra, &b[0], uint32(len(b)), &o.qty, &o.o)
	})
	return
}

// Network file descriptor.
//
// Copied from src/net/fd_posix.go
type netFD struct {
	pfd pFD

	// immutable until Close
	family      int
	sotype      int
	isConnected bool // handshake completed or use of association with peer
	net         string
	laddr       net.Addr
	raddr       net.Addr
}

//go:linkname newFD net.newFD
func newFD(sysfd windows.Handle, family, sotype int, net string) (*netFD, error)

//go:linkname netFDInit net.(*netFD).init
func netFDInit(fd *netFD) error

//go:linkname netFDClose net.(*netFD).Close
func netFDClose(fd *netFD) error

//go:linkname netFDCtrlNetwork net.(*netFD).ctrlNetwork
func netFDCtrlNetwork(fd *netFD) string

//go:linkname netFDWrite net.(*netFD).Write
func netFDWrite(fd *netFD, p []byte) (int, error)

//go:linkname netFDSetWriteDeadline net.(*netFD).SetWriteDeadline
func netFDSetWriteDeadline(fd *netFD, t time.Time) error

func (fd *netFD) init() error {
	return netFDInit(fd)
}

func (fd *netFD) Close() error {
	return netFDClose(fd)
}

func (fd *netFD) ctrlNetwork() string {
	return netFDCtrlNetwork(fd)
}

func (fd *netFD) Write(p []byte) (int, error) {
	return netFDWrite(fd, p)
}

func (fd *netFD) SetWriteDeadline(t time.Time) error {
	return netFDSetWriteDeadline(fd, t)
}

// Copied from src/net/rawconn.go
type rawConn struct {
	fd *netFD
}

func newRawConn(fd *netFD) *rawConn {
	return &rawConn{fd: fd}
}

//go:linkname rawConnControl net.(*rawConn).Control
func rawConnControl(c *rawConn, f func(uintptr)) error

//go:linkname rawConnRead net.(*rawConn).Read
func rawConnRead(c *rawConn, f func(uintptr) bool) error

//go:linkname rawConnWrite net.(*rawConn).Write
func rawConnWrite(c *rawConn, f func(uintptr) bool) error

func (c *rawConn) Control(f func(uintptr)) error {
	return rawConnControl(c, f)
}

func (c *rawConn) Read(f func(uintptr) bool) error {
	return rawConnRead(c, f)
}

func (c *rawConn) Write(f func(uintptr) bool) error {
	return rawConnWrite(c, f)
}
