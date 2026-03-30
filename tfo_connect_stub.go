//go:build !darwin && !freebsd && !linux && !(windows && tfogo_checklinkname0)

package tfo

import (
	"context"
	"net"
	"net/netip"
)

const comptimeDialNoTFO = true

func (d *Dialer) dialTFO(ctx context.Context, network, address string, b []byte) (*net.TCPConn, error) {
	if d.Fallback {
		return d.dialAndWriteTCPConn(ctx, network, address, b)
	}
	return nil, ErrPlatformUnsupported
}

func (d *Dialer) dialTCP(ctx context.Context, network string, laddr, raddr netip.AddrPort, b []byte) (*net.TCPConn, error) {
	if d.Fallback {
		return d.dialTCPAndWrite(ctx, network, laddr, raddr, b)
	}
	return nil, ErrPlatformUnsupported
}
