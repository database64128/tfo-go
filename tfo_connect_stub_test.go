//go:build !darwin && !freebsd && !linux && !(windows && tfogo_checklinkname0)

package tfo

import (
	"net/netip"
	"testing"
)

func TestDialTFO(t *testing.T) {
	s, err := newDiscardTCPServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var d Dialer
	addrPort := s.AddrPort()

	c, err := d.DialContext(t.Context(), "tcp", addrPort.String(), hello)
	if c != nil {
		t.Error("Expected nil connection")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}

	tc, err := d.DialTCP(t.Context(), "tcp", netip.AddrPort{}, addrPort, hello)
	if tc != nil {
		t.Error("Expected nil connection")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}
}
