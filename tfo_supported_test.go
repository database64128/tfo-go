//go:build darwin || freebsd || linux || windows

package tfo

import (
	"net"
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

func testClientWriteReadServerReadWriteTCPAddr(listenTCPAddr, dialLocalTCPAddr *net.TCPAddr, t *testing.T) {
	t.Logf("c->s payload: %v", helloworld)
	t.Logf("s->c payload: %v", worldhello)

	lntcp, err := ListenTCP("tcp", listenTCPAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("Started listener on", lntcp.Addr())

	ctrlCh := make(chan struct{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		t.Log("Accepted", conn.RemoteAddr())

		readUntilEOF(conn, helloworld, t)
		write(conn, world, t)
		write(conn, hello, t)
		conn.CloseWrite()
		close(ctrlCh)
	}()

	port := lntcp.Addr().(*net.TCPAddr).Port
	ip := net.IPv6loopback
	if listenTCPAddr != nil && listenTCPAddr.IP != nil {
		ip = listenTCPAddr.IP
	}

	tc, err := DialTCP("tcp", dialLocalTCPAddr, &net.TCPAddr{
		IP:   ip,
		Port: port,
	}, hello)
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	write(tc, world, t)
	tc.CloseWrite()
	readUntilEOF(tc, worldhello, t)
	<-ctrlCh
}

func TestClientWriteReadServerReadWriteTCPAddr(t *testing.T) {
	for _, c := range []struct {
		name             string
		listenTCPAddr    *net.TCPAddr
		dialLocalTCPAddr *net.TCPAddr
	}{
		{
			name:             "Unspecified",
			listenTCPAddr:    nil,
			dialLocalTCPAddr: nil,
		},
		{
			name: "IPv4Loopback",
			listenTCPAddr: &net.TCPAddr{
				IP: net.IPv4(127, 0, 0, 1),
			},
			dialLocalTCPAddr: nil,
		},
		{
			name: "IPv6Loopback",
			listenTCPAddr: &net.TCPAddr{
				IP: net.IPv6loopback,
			},
			dialLocalTCPAddr: nil,
		},
		{
			name:          "DialBind",
			listenTCPAddr: nil,
			dialLocalTCPAddr: &net.TCPAddr{
				IP: net.IPv6loopback,
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			testClientWriteReadServerReadWriteTCPAddr(c.listenTCPAddr, c.dialLocalTCPAddr, t)
		})
	}
}
