//go:build darwin || freebsd || windows
// +build darwin freebsd windows

package tfo

import (
	"context"
	"net"
)

func (d *TFODialer) dialTFOContext(ctx context.Context, network, address string) (net.Conn, error) {
	var laddr *net.TCPAddr
	if d.LocalAddr != nil {
		la, ok := d.LocalAddr.(*net.TCPAddr)
		if !ok {
			return nil, &net.OpError{
				Op:     "dial",
				Net:    network,
				Source: nil,
				Addr:   nil,
				Err: &net.AddrError{
					Err:  "mismatched local address type",
					Addr: d.LocalAddr.String(),
				},
			}
		}
		laddr = la
	}
	//FIXME: Implement happy eyeballs.
	raddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}
	return dialTFO(network, laddr, raddr)
}
