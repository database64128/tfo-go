package tfo

import (
	"net"
	"sync"
	"syscall"
	_ "unsafe"

	"golang.org/x/sys/windows"
)

//go:linkname sockaddrToTCP net.sockaddrToTCP
func sockaddrToTCP(sa syscall.Sockaddr) net.Addr

//go:linkname runtime_pollServerInit internal/poll.runtime_pollServerInit
func runtime_pollServerInit()

//go:linkname runtime_pollOpen internal/poll.runtime_pollOpen
func runtime_pollOpen(fd uintptr) (uintptr, int)

// Copied from src/internal/poll/fd_poll_runtime.go
var serverInit sync.Once

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
	Sysfd syscall.Handle

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

func (fd *pFD) init() error {
	serverInit.Do(runtime_pollServerInit)
	ctx, errno := runtime_pollOpen(uintptr(fd.Sysfd))
	if errno != 0 {
		return syscall.Errno(errno)
	}
	fd.pd = ctx
	fd.rop.mode = 'r'
	fd.wop.mode = 'w'
	fd.rop.fd = fd
	fd.wop.fd = fd
	fd.rop.runtimeCtx = fd.pd
	fd.wop.runtimeCtx = fd.pd
	return nil
}

func (fd *pFD) ConnectEx(ra syscall.Sockaddr, b []byte) (n int, err error) {
	fd.wop.sa = ra
	n, err = execIO(&fd.wop, func(o *operation) error {
		return syscall.ConnectEx(o.fd.Sysfd, o.sa, &b[0], uint32(len(b)), &o.qty, &o.o)
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

func (fd *netFD) ctrlNetwork() string {
	if fd.net == "tcp4" || fd.family == windows.AF_INET {
		return "tcp4"
	}
	return "tcp6"
}

//go:linkname newFD net.newFD
func newFD(sysfd syscall.Handle, family, sotype int, net string) (*netFD, error)

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
