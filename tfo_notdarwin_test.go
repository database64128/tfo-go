//go:build !darwin
// +build !darwin

package tfo

import (
	"context"
	"syscall"
	"testing"
)

func TestTFOListenConfigCtrlFn(t *testing.T) {
	var success bool
	lc := TFOListenConfig{}
	lc.Control = func(network, address string, c syscall.RawConn) error {
		success = true
		return nil
	}
	ln, err := lc.Listen(context.Background(), "tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if !success {
		t.Fail()
	}
}
