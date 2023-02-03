//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"net"
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
	c, err := Dial("tcp", "example.com:443", hello)
	if c != nil {
		t.Error("Expected nil connection")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv4(1, 1, 1, 1),
		Port: 443,
	}, hello)
	if tc != nil {
		t.Error("Expected nil connection")
	}
	if err != ErrPlatformUnsupported {
		t.Errorf("Expected ErrPlatformUnsupported, got %v", err)
	}
}

func TestListenCtrlFn(t *testing.T) {
	testListenCtrlFn(t, defaultListenConfigNoTFO)
}

func TestDialCtrlFn(t *testing.T) {
	testDialCtrlFn(t, defaultDialerNoTFO)
	testDialCtrlCtxFn(t, defaultDialerNoTFO)
	testDialCtrlCtxFnSupersedesCtrlFn(t, defaultDialerNoTFO)
}

func TestAddrFunctions(t *testing.T) {
	testAddrFunctions(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
}

func TestClientWriteReadServerReadWrite(t *testing.T) {
	testClientWriteReadServerReadWrite(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
}

func TestServerWriteReadClientReadWrite(t *testing.T) {
	testServerWriteReadClientReadWrite(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
}

func TestClientServerReadFrom(t *testing.T) {
	testClientServerReadFrom(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
}

func TestSetDeadline(t *testing.T) {
	testSetDeadline(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
}
