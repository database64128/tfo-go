//go:build darwin || freebsd || linux || windows

package tfo

import (
	"testing"
)

func TestListenCtrlFn(t *testing.T) {
	t.Run("TFO", func(t *testing.T) {
		testListenCtrlFn(t, defaultListenConfig)
	})
	t.Run("NoTFO", func(t *testing.T) {
		testListenCtrlFn(t, defaultListenConfigNoTFO)
	})
}

func TestDialCtrlFn(t *testing.T) {
	t.Run("TFO", func(t *testing.T) {
		testDialCtrlFn(t, defaultDialer)
		testDialCtrlCtxFn(t, defaultDialer)
		testDialCtrlCtxFnSupersedesCtrlFn(t, defaultDialer)
	})
	t.Run("NoTFO", func(t *testing.T) {
		testDialCtrlFn(t, defaultDialerNoTFO)
		testDialCtrlCtxFn(t, defaultDialerNoTFO)
		testDialCtrlCtxFnSupersedesCtrlFn(t, defaultDialerNoTFO)
	})
}

func TestAddrFunctions(t *testing.T) {
	t.Run("TFO", func(t *testing.T) {
		testAddrFunctions(t, defaultListenConfig, defaultDialer)
	})
	t.Run("NoTFO", func(t *testing.T) {
		testAddrFunctions(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
	})
}

func TestClientWriteReadServerReadWrite(t *testing.T) {
	t.Run("TFO", func(t *testing.T) {
		testClientWriteReadServerReadWrite(t, defaultListenConfig, defaultDialer)
	})
	t.Run("NoTFO", func(t *testing.T) {
		testClientWriteReadServerReadWrite(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
	})
}

func TestServerWriteReadClientReadWrite(t *testing.T) {
	t.Run("TFO", func(t *testing.T) {
		testServerWriteReadClientReadWrite(t, defaultListenConfig, defaultDialer)
	})
	t.Run("NoTFO", func(t *testing.T) {
		testServerWriteReadClientReadWrite(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
	})
}

func TestClientServerReadFrom(t *testing.T) {
	t.Run("TFO", func(t *testing.T) {
		testClientServerReadFrom(t, defaultListenConfig, defaultDialer)
	})
	t.Run("NoTFO", func(t *testing.T) {
		testClientServerReadFrom(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
	})
}

func TestSetDeadline(t *testing.T) {
	t.Run("TFO", func(t *testing.T) {
		testSetDeadline(t, defaultListenConfig, defaultDialer)
	})
	t.Run("NoTFO", func(t *testing.T) {
		testSetDeadline(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
	})
}
