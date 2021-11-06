package tfo

import (
	"context"
	"syscall"
	"testing"
)

func TestTFOListenConfigCtrlFn(t *testing.T) {
	var success bool
	lc := ListenConfig{}
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

func TestTFODialerCtrlFn(t *testing.T) {
	var success bool
	d := Dialer{}
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
