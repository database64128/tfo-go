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

func (d *Dialer) dialTFOContext(ctx context.Context, network, address string, b []byte) (net.Conn, error) {
	ld := *d
	ld.Control = func(network, address string, c syscall.RawConn) (err error) {
		if ctrlFn := d.Control; ctrlFn != nil {
			if err = ctrlFn(network, address, c); err != nil {
				return
			}
		}
		return c.Control(func(fd uintptr) {
			err = SetTFODialer(fd)
		})
	}
	c, err := ld.Dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if _, err = c.Write(b); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func dialTFO(network string, laddr, raddr *net.TCPAddr, b []byte, ctrlFn func(string, string, syscall.RawConn) error) (*net.TCPConn, error) {
	d := Dialer{Dialer: net.Dialer{LocalAddr: laddr, Control: ctrlFn}}
	c, err := d.dialTFOContext(context.Background(), network, raddr.String(), b)
	if err != nil {
		return nil, err
	}
	return c.(*net.TCPConn), nil
}
