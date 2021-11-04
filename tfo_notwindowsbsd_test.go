//go:build !darwin && !freebsd && !windows
// +build !darwin,!freebsd,!windows

package tfo

import (
	"syscall"
	"testing"
)

func TestTFODialerCtrlFn(t *testing.T) {
	var success bool
	d := TFODialer{}
	d.Control = func(network, address string, c syscall.RawConn) error {
		success = true
		return nil
	}
	c, err := d.Dial("tcp", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !success {
		t.Fail()
	}
}
