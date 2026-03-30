//go:build darwin || freebsd || (windows && tfogo_checklinkname0)

package tfo

import (
	"context"
	"net"
	"net/netip"
)

func (d *Dialer) dialTFO(ctx context.Context, network, address string, b []byte) (*net.TCPConn, error) {
	if d.Fallback && runtimeDialTFOSupport.load() == dialTFOSupportNone {
		return d.dialAndWriteTCPConn(ctx, network, address, b)
	}
	return d.dialTFOFromSocket(ctx, network, address, b)
}

func (d *Dialer) dialTCP(ctx context.Context, network string, laddr, raddr netip.AddrPort, b []byte) (*net.TCPConn, error) {
	if d.Fallback && runtimeDialTFOSupport.load() == dialTFOSupportNone {
		return d.dialTCPAndWrite(ctx, network, laddr, raddr, b)
	}
	return d.dialTCPAddrFromSocket(ctx, network, laddr, raddr, b)
}
