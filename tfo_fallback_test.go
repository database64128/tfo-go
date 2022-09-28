//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"net"
	"testing"
)

func TestListener(t *testing.T) {
	ln, err := Listen("tcp", "")
	if ln != nil {
		t.Error("Expected nil listener")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}

	lntcp, err := ListenTCP("tcp", nil)
	if lntcp != nil {
		t.Error("Expected nil listener")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}
}

func TestDialer(t *testing.T) {
	foobar := []byte{'f', 'o', 'o', 'b', 'a', 'r'}

	c, err := Dial("tcp", "example.com:443", foobar)
	if c != nil {
		t.Error("Expected nil connection")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv4(1, 1, 1, 1),
		Port: 443,
	}, foobar)
	if tc != nil {
		t.Error("Expected nil connection")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}
}
