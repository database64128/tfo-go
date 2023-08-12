//go:build darwin || freebsd || linux || windows

package tfo

import (
	"testing"
)

func TestListenCtrlFn(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testListenCtrlFn(t, c.listenConfig)
		})
	}
}

func TestDialCtrlFn(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testDialCtrlFn(t, c.dialer)
			testDialCtrlCtxFn(t, c.dialer)
			testDialCtrlCtxFnSupersedesCtrlFn(t, c.dialer)
		})
	}
}

func TestAddrFunctions(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testAddrFunctions(t, c.listenConfig, c.dialer)
		})
	}
}

func TestClientWriteReadServerReadWrite(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testClientWriteReadServerReadWrite(t, c.listenConfig, c.dialer)
		})
	}
}

func TestServerWriteReadClientReadWrite(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testServerWriteReadClientReadWrite(t, c.listenConfig, c.dialer)
		})
	}
}

func TestClientServerReadFrom(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testClientServerReadFrom(t, c.listenConfig, c.dialer)
		})
	}
}

func TestSetDeadline(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testSetDeadline(t, c.listenConfig, c.dialer)
		})
	}
}
