//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"net"
	"testing"
)

// fallbackCases references cases that have TFO disabled.
// The index must be updated if [cases] is updated.
var fallbackCases = cases[3:]

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
	for _, c := range fallbackCases {
		t.Run(c.name, func(t *testing.T) {
			testListenCtrlFn(t, c.listenConfig)
		})
	}
}

func TestDialCtrlFn(t *testing.T) {
	for _, c := range fallbackCases {
		t.Run(c.name, func(t *testing.T) {
			testDialCtrlFn(t, c.dialer)
			testDialCtrlCtxFn(t, c.dialer)
			testDialCtrlCtxFnSupersedesCtrlFn(t, c.dialer)
		})
	}
}

func TestAddrFunctions(t *testing.T) {
	for _, c := range fallbackCases {
		t.Run(c.name, func(t *testing.T) {
			testAddrFunctions(t, c.listenConfig, c.dialer)
		})
	}
}

func TestClientWriteReadServerReadWrite(t *testing.T) {
	for _, c := range fallbackCases {
		t.Run(c.name, func(t *testing.T) {
			testClientWriteReadServerReadWrite(t, c.listenConfig, c.dialer)
		})
	}
}

func TestServerWriteReadClientReadWrite(t *testing.T) {
	for _, c := range fallbackCases {
		t.Run(c.name, func(t *testing.T) {
			testServerWriteReadClientReadWrite(t, c.listenConfig, c.dialer)
		})
	}
}

func TestClientServerReadFrom(t *testing.T) {
	for _, c := range fallbackCases {
		t.Run(c.name, func(t *testing.T) {
			testClientServerReadFrom(t, c.listenConfig, c.dialer)
		})
	}
}

func TestSetDeadline(t *testing.T) {
	for _, c := range fallbackCases {
		t.Run(c.name, func(t *testing.T) {
			testSetDeadline(t, c.listenConfig, c.dialer)
		})
	}
}
