//go:build !linux
// +build !linux

package tfo

import (
	"net"
	"testing"
)

func TestDialTCPMismatchedLocalRemoteAddr(t *testing.T) {
	tc, err := DialTCP("tcp", &net.TCPAddr{
		IP: net.IPv6loopback,
	}, &net.TCPAddr{
		IP:   net.IPv4(1, 1, 1, 1),
		Port: 443,
	})
	if tc != nil || err != ErrMismatchedAddressFamily {
		t.Fail()
	}
}
