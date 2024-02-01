//go:build windows && !go1.23

package tfo

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// operation contains superset of data necessary to perform all async IO.
//
// Copied from src/internal/poll/fd_windows.go
type operation struct {
	// Used by IOCP interface, it must be first field
	// of the struct, as our code rely on it.
	o syscall.Overlapped

	// fields used by runtime.netpoll
	runtimeCtx uintptr
	mode       int32
	errno      int32
	qty        uint32

	// fields used only by net package
	fd     *pFD
	buf    syscall.WSABuf
	msg    windows.WSAMsg
	sa     syscall.Sockaddr
	rsa    *syscall.RawSockaddrAny
	rsan   int32
	handle syscall.Handle
	flags  uint32
	bufs   []syscall.WSABuf
}
