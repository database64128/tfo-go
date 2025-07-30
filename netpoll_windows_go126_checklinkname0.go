//go:build windows && go1.26 && tfogo_checklinkname0

package tfo

import (
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
	buf  windows.WSABuf
	rsa  *windows.RawSockaddrAny
	bufs []windows.WSABuf
}

//go:linkname execIO internal/poll.execIO
func execIO(fd *pFD, o *operation, submit func(o *operation) (uint32, error)) (int, error)

func (fd *pFD) ConnectEx(ra windows.Sockaddr, b []byte) (n int, err error) {
	n, err = execIO(fd, &fd.wop, func(o *operation) (qty uint32, err error) {
		err = windows.ConnectEx(fd.Sysfd, ra, &b[0], uint32(len(b)), &qty, &o.o)
		return qty, err
	})
	return
}
