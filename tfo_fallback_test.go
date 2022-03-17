//go:build !darwin && !freebsd && !linux && !windows

package tfo

import (
	"net"
	"testing"
)

func TestListener(t *testing.T) {
	ln, err := Listen("tcp", "")
	if err != ErrPlatformUnsupported {
		t.FailNow()
		if err != nil {
			t.Fatal(err)
		}
	}
	err = ln.Close()
	if err != nil {
		t.Fatal(err)
	}

	lntcp, err := ListenTCP("tcp", nil)
	if err != ErrPlatformUnsupported {
		t.FailNow()
		if err != nil {
			t.Fatal(err)
		}
	}
	err = lntcp.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDialer(t *testing.T) {
	c, err := Dial("tcp", "example.com:443")
	if err != ErrPlatformUnsupported {
		t.FailNow()
		if err != nil {
			t.Fatal(err)
		}
	}
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv4(1, 1, 1, 1),
		Port: 443,
	})
	if err != ErrPlatformUnsupported {
		t.FailNow()
		if err != nil {
			t.Fatal(err)
		}
	}
	err = tc.Close()
	if err != nil {
		t.Fatal(err)
	}
}
