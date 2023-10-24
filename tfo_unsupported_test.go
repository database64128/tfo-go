//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"context"
	"testing"
)

func TestListenTFO(t *testing.T) {
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

func TestDialTFO(t *testing.T) {
	s, err := newDiscardTCPServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	addr := s.Addr()

	c, err := Dial("tcp", addr.String(), hello)
	if c != nil {
		t.Error("Expected nil connection")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}

	tc, err := DialTCP("tcp", nil, addr, hello)
	if tc != nil {
		t.Error("Expected nil connection")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}
}
