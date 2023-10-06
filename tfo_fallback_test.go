//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"context"
	"testing"
)

const discardTCPServerDisableTFO = true

// tfoDisabledCases references cases that have TFO disabled.
var tfoDisabledCases []testCase

func init() {
	tfoDisabledCases = make([]testCase, 0, len(cases)/2)
	for _, c := range cases {
		if !c.listenConfig.tfoDisabled() || !c.dialer.DisableTFO {
			continue
		}
		tfoDisabledCases = append(tfoDisabledCases, c)
	}
}

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

func TestListenCtrlFn(t *testing.T) {
	for _, c := range tfoDisabledCases {
		t.Run(c.name, func(t *testing.T) {
			testListenCtrlFn(t, c.listenConfig)
		})
	}
}

func TestDialCtrlFn(t *testing.T) {
	s, err := newDiscardTCPServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	address := s.Addr().String()

	for _, c := range tfoDisabledCases {
		t.Run(c.name, func(t *testing.T) {
			testDialCtrlFn(t, c.dialer, address)
			testDialCtrlCtxFn(t, c.dialer, address)
			testDialCtrlCtxFnSupersedesCtrlFn(t, c.dialer, address)
		})
	}
}

func TestAddrFunctions(t *testing.T) {
	for _, c := range tfoDisabledCases {
		t.Run(c.name, func(t *testing.T) {
			testAddrFunctions(t, c.listenConfig, c.dialer)
		})
	}
}

func TestClientWriteReadServerReadWrite(t *testing.T) {
	for _, c := range tfoDisabledCases {
		t.Run(c.name, func(t *testing.T) {
			testClientWriteReadServerReadWrite(t, c.listenConfig, c.dialer)
		})
	}
}

func TestServerWriteReadClientReadWrite(t *testing.T) {
	for _, c := range tfoDisabledCases {
		t.Run(c.name, func(t *testing.T) {
			testServerWriteReadClientReadWrite(t, c.listenConfig, c.dialer)
		})
	}
}

func TestClientServerReadFrom(t *testing.T) {
	for _, c := range tfoDisabledCases {
		t.Run(c.name, func(t *testing.T) {
			testClientServerReadFrom(t, c.listenConfig, c.dialer)
		})
	}
}

func TestSetDeadline(t *testing.T) {
	for _, c := range tfoDisabledCases {
		t.Run(c.name, func(t *testing.T) {
			testSetDeadline(t, c.listenConfig, c.dialer)
		})
	}
}
