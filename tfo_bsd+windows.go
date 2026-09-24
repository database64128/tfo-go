//go:build darwin || freebsd || (windows && tfogo_checklinkname0)

package tfo

import (
	"context"
	"net"
	"net/netip"
)

func (d *Dialer) dial(ctx context.Context, network, address string, b []byte) (net.Conn, error) {
	if d.Fallback && runtimeDialTFOSupport.Load() == dialTFOSupportNone {
		return d.dialAndWrite(ctx, network, address, b)
	}
	return d.dialFromSocket(ctx, network, address, b)
}

func (d *Dialer) dialTCP(ctx context.Context, network string, laddr, raddr netip.AddrPort, b []byte) (*net.TCPConn, error) {
	if d.Fallback && runtimeDialTFOSupport.Load() == dialTFOSupportNone {
		return d.dialTCPAndWrite(ctx, network, laddr, raddr, b)
	}
	return d.dialTCPFromSocket(ctx, network, laddr, raddr, b)
}
