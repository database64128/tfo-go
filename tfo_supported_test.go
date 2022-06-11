//go:build darwin || freebsd || linux || windows

package tfo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

var (
	hello              = []byte{'h', 'e', 'l', 'l', 'o'}
	world              = []byte{'w', 'o', 'r', 'l', 'd'}
	helloworld         = []byte{'h', 'e', 'l', 'l', 'o', 'w', 'o', 'r', 'l', 'd'}
	worldhello         = []byte{'w', 'o', 'r', 'l', 'd', 'h', 'e', 'l', 'l', 'o'}
	helloWorldSentence = []byte{'h', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd', '!', '\n'}
)

func TestListen(t *testing.T) {
	ln, err := Listen("tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	err = ln.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestListenTCP(t *testing.T) {
	lntcp, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = lntcp.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDial(t *testing.T) {
	c, err := Dial("tcp", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDialTCP(t *testing.T) {
	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv4(1, 1, 1, 1),
		Port: 443,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = tc.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func write(w io.Writer, data []byte, t *testing.T) {
	dataLen := len(data)
	n, err := w.Write(data)
	if err != nil {
		t.Error(err)
		return
	}
	if n != dataLen {
		t.Errorf("written %d bytes, should have written %d bytes", n, dataLen)
	}
}

func writeWithReadFrom(w io.ReaderFrom, data []byte, t *testing.T) {
	r := bytes.NewReader(data)
	n, err := w.ReadFrom(r)
	if err != nil {
		t.Error(err)
	}
	bytesWritten := int(n)
	dataLen := len(data)
	if bytesWritten != dataLen {
		t.Errorf("written %d bytes, should have written %d bytes", bytesWritten, dataLen)
	}
}

func readExactlyOneByte(r io.Reader, expectedByte byte, t *testing.T) {
	b := make([]byte, 1)
	n, err := r.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("read %d bytes, expected 1 byte", n)
	}
	if b[0] != expectedByte {
		t.Fatalf("read unexpected byte: '%c', expected '%c'", b[0], expectedByte)
	}
}

func readUntilEOF(r io.Reader, expectedData []byte, t *testing.T) {
	b, err := io.ReadAll(r)
	if err != nil {
		t.Error(err)
		return
	}
	if !bytes.Equal(b, expectedData) {
		t.Errorf("read data %v does not equal original data %v", b, expectedData)
	}
}

func TestClientWriteReadServerReadWriteAddress(t *testing.T) {
	t.Logf("c->s payload: %v", helloworld)
	t.Logf("s->c payload: %v", worldhello)

	var lc ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	lntcp := ln.(*net.TCPListener)
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	ctrlCh := make(chan struct{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		t.Log("accepted", conn.RemoteAddr())

		readUntilEOF(conn, helloworld, t)
		write(conn, world, t)
		write(conn, hello, t)
		conn.CloseWrite()
		ctrlCh <- struct{}{}
	}()

	var dialer Dialer
	c, err := dialer.Dial("tcp", fmt.Sprintf("localhost:%d", ln.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatal(err)
	}
	tc := c.(Conn)
	defer tc.Close()

	write(tc, hello, t)
	write(tc, world, t)
	tc.CloseWrite()
	readUntilEOF(tc, worldhello, t)
	<-ctrlCh
}

func testClientWriteReadServerReadWriteTCPAddr(listenTCPAddr, dialLocalTCPAddr *net.TCPAddr, t *testing.T) {
	t.Logf("c->s payload: %v", helloworld)
	t.Logf("s->c payload: %v", worldhello)

	lntcp, err := ListenTCP("tcp", listenTCPAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	ctrlCh := make(chan struct{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		t.Log("accepted", conn.RemoteAddr())

		readUntilEOF(conn, helloworld, t)
		write(conn, world, t)
		write(conn, hello, t)
		conn.CloseWrite()
		ctrlCh <- struct{}{}
	}()

	port := lntcp.Addr().(*net.TCPAddr).Port
	ip := net.IPv6loopback
	if listenTCPAddr != nil && listenTCPAddr.IP != nil {
		ip = listenTCPAddr.IP
	}

	tc, err := DialTCP("tcp", dialLocalTCPAddr, &net.TCPAddr{
		IP:   ip,
		Port: port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	write(tc, hello, t)
	write(tc, world, t)
	tc.CloseWrite()
	readUntilEOF(tc, worldhello, t)
	<-ctrlCh
}

func TestClientWriteReadServerReadWriteUnspecified(t *testing.T) {
	testClientWriteReadServerReadWriteTCPAddr(nil, nil, t)
}

func TestClientWriteReadServerReadWriteIPv4Loopback(t *testing.T) {
	testClientWriteReadServerReadWriteTCPAddr(&net.TCPAddr{
		IP: net.IPv4(127, 0, 0, 1),
	}, nil, t)
}

func TestClientWriteReadServerReadWriteIPv6Loopback(t *testing.T) {
	testClientWriteReadServerReadWriteTCPAddr(&net.TCPAddr{
		IP: net.IPv6loopback,
	}, nil, t)
}

func TestClientWriteReadServerReadWriteDialBind(t *testing.T) {
	testClientWriteReadServerReadWriteTCPAddr(nil, &net.TCPAddr{
		IP: net.IPv6loopback,
	}, t)
}

func TestServerWriteReadClientReadWrite(t *testing.T) {
	t.Logf("c->s payload: %v", helloworld)
	t.Logf("s->c payload: %v", worldhello)

	lntcp, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	ctrlCh := make(chan struct{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			t.Error(err)
			return
		}
		t.Log("accepted", conn.RemoteAddr())
		defer conn.Close()

		write(conn, world, t)
		write(conn, hello, t)
		conn.CloseWrite()
		readUntilEOF(conn, helloworld, t)
		ctrlCh <- struct{}{}
	}()

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv6loopback,
		Port: lntcp.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	readUntilEOF(tc, worldhello, t)
	write(tc, hello, t)
	write(tc, world, t)
	tc.CloseWrite()
	<-ctrlCh
}

func TestClientServerReadFrom(t *testing.T) {
	t.Logf("c->s payload: %v", helloworld)
	t.Logf("s->c payload: %v", worldhello)

	lntcp, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	ctrlCh := make(chan struct{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		t.Log("accepted", conn.RemoteAddr())

		readUntilEOF(conn, helloworld, t)
		writeWithReadFrom(conn, world, t)
		writeWithReadFrom(conn, hello, t)
		conn.CloseWrite()
		ctrlCh <- struct{}{}
	}()

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv6loopback,
		Port: lntcp.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	writeWithReadFrom(tc, hello, t)
	writeWithReadFrom(tc, world, t)
	tc.CloseWrite()
	readUntilEOF(tc, worldhello, t)
	<-ctrlCh
}

func TestSetDeadline(t *testing.T) {
	t.Logf("payload: %v", helloWorldSentence)

	lntcp, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	ctrlCh := make(chan struct{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			t.Error(err)
			return
		}
		t.Log("accepted", conn.RemoteAddr())
		defer conn.Close()

		write(conn, helloWorldSentence, t)
		readUntilEOF(conn, []byte{'h', 'l', 'l', ','}, t)
		ctrlCh <- struct{}{}
	}()

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv6loopback,
		Port: lntcp.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	b := make([]byte, 1)

	// SetReadDeadline
	readExactlyOneByte(tc, 'h', t)
	if err := tc.SetReadDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := tc.Read(b); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(n, err)
	}
	if err := tc.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	readExactlyOneByte(tc, 'e', t)

	// SetWriteDeadline
	write(tc, helloWorldSentence[:1], t)
	if err := tc.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := tc.Write(helloWorldSentence[1:2]); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(n, err)
	}
	if err := tc.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	write(tc, helloWorldSentence[2:3], t)

	// SetDeadline
	readExactlyOneByte(tc, 'l', t)
	write(tc, helloWorldSentence[3:4], t)
	if err := tc.SetDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := tc.Read(b); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(err)
	}
	if n, err := tc.Write(helloWorldSentence[4:5]); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(n, err)
	}
	if err := tc.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	readExactlyOneByte(tc, 'l', t)
	write(tc, helloWorldSentence[5:6], t)

	tc.CloseWrite()
	<-ctrlCh
}
