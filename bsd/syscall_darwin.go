package bsd

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Do the interface allocations only once for common
// Errno values.
var (
	errEAGAIN error = syscall.EAGAIN
	errEINVAL error = syscall.EINVAL
	errENOENT error = syscall.ENOENT
)

// errnoErr returns common boxed Errno values, to prevent
// allocations at runtime.
func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	case unix.EAGAIN:
		return errEAGAIN
	case unix.EINVAL:
		return errEINVAL
	case unix.ENOENT:
		return errENOENT
	}
	return e
}

func sockaddr(sa unix.Sockaddr) (unsafe.Pointer, uint32, error) {
	switch sa := sa.(type) {
	case nil:
		return nil, 0, nil
	case *unix.SockaddrInet4:
		return sockaddr4(sa)
	case *unix.SockaddrInet6:
		return sockaddr6(sa)
	default:
		return nil, 0, errors.New("unsupported unix.Sockaddr")
	}
}

func sockaddr4(sa *unix.SockaddrInet4) (unsafe.Pointer, uint32, error) {
	if sa.Port < 0 || sa.Port > 0xFFFF {
		return nil, 0, unix.EINVAL
	}
	raw := unix.RawSockaddrInet4{
		Len:    unix.SizeofSockaddrInet4,
		Family: unix.AF_INET,
		Addr:   sa.Addr,
	}
	p := (*[2]byte)(unsafe.Pointer(&raw.Port))
	p[0] = byte(sa.Port >> 8)
	p[1] = byte(sa.Port)
	return unsafe.Pointer(&raw), uint32(raw.Len), nil
}

func sockaddr6(sa *unix.SockaddrInet6) (unsafe.Pointer, uint32, error) {
	if sa.Port < 0 || sa.Port > 0xFFFF {
		return nil, 0, unix.EINVAL
	}
	raw := unix.RawSockaddrInet6{
		Len:    unix.SizeofSockaddrInet6,
		Family: unix.AF_INET6,
		Addr:   sa.Addr,
	}
	p := (*[2]byte)(unsafe.Pointer(&raw.Port))
	p[0] = byte(sa.Port >> 8)
	p[1] = byte(sa.Port)
	raw.Scope_id = sa.ZoneId
	return unsafe.Pointer(&raw), uint32(raw.Len), nil
}

type sa_endpoints_t struct {
	sae_srcif      uint
	sae_srcaddr    unsafe.Pointer
	sae_srcaddrlen uint32
	sae_dstaddr    unsafe.Pointer
	sae_dstaddrlen uint32
}

const (
	SAE_ASSOCID_ANY              = 0
	CONNECT_RESUME_ON_READ_WRITE = 0x1
	CONNECT_DATA_IDEMPOTENT      = 0x2
	CONNECT_DATA_AUTHENTICATED   = 0x4
)

// Connectx enables TFO if a non-empty buf is passed.
// If an empty buf is passed, TFO is not enabled.
func Connectx(s int, srcif uint, from unix.Sockaddr, to unix.Sockaddr, buf []byte) (uint, error) {
	from_ptr, from_n, err := sockaddr(from)
	if err != nil {
		return 0, err
	}

	to_ptr, to_n, err := sockaddr(to)
	if err != nil {
		return 0, err
	}

	sae := sa_endpoints_t{
		sae_srcif:      srcif,
		sae_srcaddr:    from_ptr,
		sae_srcaddrlen: from_n,
		sae_dstaddr:    to_ptr,
		sae_dstaddrlen: to_n,
	}

	var (
		flags  uint
		iov    *unix.Iovec
		iovcnt uint
	)

	if len(buf) > 0 {
		flags = CONNECT_DATA_IDEMPOTENT
		iov = &unix.Iovec{
			Base: &buf[0],
			Len:  uint64(len(buf)),
		}
		iovcnt = 1
	}

	var bytesSent uint

	r1, _, e1 := unix.Syscall9(unix.SYS_CONNECTX,
		uintptr(s),
		uintptr(unsafe.Pointer(&sae)),
		SAE_ASSOCID_ANY,
		uintptr(flags),
		uintptr(unsafe.Pointer(iov)),
		uintptr(iovcnt),
		uintptr(unsafe.Pointer(&bytesSent)),
		0,
		0)
	ret := int(r1)
	if ret == -1 {
		err = errnoErr(e1)
	}
	return bytesSent, err
}
