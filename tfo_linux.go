package tfo

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// TCPFastopenQueueLength sets the maximum number of total pending TFO connection requests.
// ref: https://datatracker.ietf.org/doc/html/rfc7413#section-5.1
// We default to 4096 to align with listener's default backlog.
// Change to a lower value if your application is vulnerable to such attacks.
const TCPFastopenQueueLength = 4096

func SetTFOListener(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, TCPFastopenQueueLength)
}

func SetTFODialer(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN_CONNECT, 1)
}

func dialTFO(network string, laddr, raddr *net.TCPAddr, ctrlFn func(string, string, syscall.RawConn) error) (TFOConn, error) {
	var innerErr error
	d := net.Dialer{
		LocalAddr: laddr,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				innerErr = SetTFODialer(fd)
			})
		},
	}
	c, err := d.DialContext(context.Background(), network, raddr.String())
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: opAddr(laddr), Addr: opAddr(raddr), Err: err}
	}
	return c.(TFOConn), innerErr
}
