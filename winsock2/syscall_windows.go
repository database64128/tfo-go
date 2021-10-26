package winsock2

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Do the interface allocations only once for common
// Errno values.
const (
	errnoERROR_IO_PENDING = 997

	socket_error = uintptr(^uint32(0))
)

var (
	errERROR_IO_PENDING error = syscall.Errno(errnoERROR_IO_PENDING)
	errERROR_EINVAL     error = syscall.EINVAL

	modws2_32          = windows.NewLazySystemDLL("ws2_32.dll")
	procWSACreateEvent = modws2_32.NewProc("WSACreateEvent")
	procsend           = modws2_32.NewProc("send")
	procrecv           = modws2_32.NewProc("recv")
)

// errnoErr returns common boxed Errno values, to prevent
// allocations at runtime.
func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return errERROR_EINVAL
	case errnoERROR_IO_PENDING:
		return errERROR_IO_PENDING
	}
	// TODO: add more here, after collecting data on the common
	// error values see on Windows. (perhaps when running
	// all.bat?)
	return e
}

func WSACreateEvent() (windows.Handle, error) {
	efd, _, err := syscall.Syscall(procWSACreateEvent.Addr(), 0, 0, 0, 0)
	if efd == 0 {
		return 0, errnoErr(err)
	}
	return windows.Handle(efd), nil
}

func Send(s windows.Handle, buf []byte, flags int32) (n int32, err error) {
	r1, _, e1 := syscall.Syscall6(procsend.Addr(), 4, uintptr(s), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(flags), 0, 0)
	if r1 == socket_error {
		err = errnoErr(e1)
		return
	}
	n = int32(r1)
	return
}

func Recv(s windows.Handle, buf []byte, flags int32) (n int32, err error) {
	r1, _, e1 := syscall.Syscall6(procrecv.Addr(), 4, uintptr(s), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(flags), 0, 0)
	if r1 == socket_error {
		err = errnoErr(e1)
		return
	}
	n = int32(r1)
	return
}
