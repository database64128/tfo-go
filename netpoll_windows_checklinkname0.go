//go:build windows && (!go1.23 || (go1.23 && tfogo_checklinkname0))

package tfo

import (
	"net"
	"sync"
	"time"
	_ "unsafe"

	"golang.org/x/sys/windows"
)

//go:linkname execIO internal/poll.execIO
func execIO(o *operation, submit func(o *operation) error) (int, error)

// pFD is a file descriptor. The net and os packages embed this type in
// a larger type representing a network connection or OS file.
//
// Stay in sync with FD in src/internal/poll/fd_windows.go
type pFD struct {
	fdmuS uint64
	fdmuR uint32
	fdmuW uint32

	// System file descriptor. Immutable until Close.
	Sysfd windows.Handle

	// Read operation.
	rop operation
	// Write operation.
	wop operation

	// I/O poller.
	pd uintptr

	// Used to implement pread/pwrite.
	l sync.Mutex

	// For console I/O.
	lastbits       []byte   // first few bytes of the last incomplete rune in last write
	readuint16     []uint16 // buffer to hold uint16s obtained with ReadConsole
	readbyte       []byte   // buffer to hold decoding of readuint16 from utf16 to utf8
	readbyteOffset int      // readbyte[readOffset:] is yet to be consumed with file.Read

	// Semaphore signaled when file is closed.
	csema uint32

	skipSyncNotif bool

	// Whether this is a streaming descriptor, as opposed to a
	// packet-based descriptor like a UDP socket.
	IsStream bool

	// Whether a zero byte read indicates EOF. This is false for a
	// message based socket connection.
	ZeroReadIsEOF bool

	// Whether this is a file rather than a network socket.
	isFile bool

	// The kind of this file.
	kind byte
}

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
