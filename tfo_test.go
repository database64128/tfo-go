package tfo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

var (
	defaultDialer            Dialer
	defaultDialerNoTFO       = Dialer{DisableTFO: true}
	defaultListenConfig      ListenConfig
	defaultListenConfigNoTFO = ListenConfig{DisableTFO: true}
)

var (
	hello              = []byte{'h', 'e', 'l', 'l', 'o'}
	world              = []byte{'w', 'o', 'r', 'l', 'd'}
	helloworld         = []byte{'h', 'e', 'l', 'l', 'o', 'w', 'o', 'r', 'l', 'd'}
	worldhello         = []byte{'w', 'o', 'r', 'l', 'd', 'h', 'e', 'l', 'l', 'o'}
	helloWorldSentence = []byte{'h', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd', '!', '\n'}
)

func testListenDialUDP(t *testing.T, lc ListenConfig, d Dialer) {
	pc, err := lc.ListenPacket(context.Background(), "udp", "[::1]:")
	if err != nil {
		t.Fatal(err)
	}
	uc := pc.(*net.UDPConn)
	defer uc.Close()

	c, err := d.Dial("udp", uc.LocalAddr().String(), hello)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	b := make([]byte, 5)
	n, _, err := uc.ReadFromUDPAddrPort(b)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("Expected 5 bytes, got %d", n)
	}
	if !bytes.Equal(b, hello) {
		t.Fatalf("Expected %v, got %v", hello, b)
	}
}

// TestListenDialUDP ensures that the UDP capabilities of [ListenConfig] and
// [Dialer] are not affected by this package.
func TestListenDialUDP(t *testing.T) {
	testListenDialUDP(t, defaultListenConfig, defaultDialer)
	testListenDialUDP(t, defaultListenConfigNoTFO, defaultDialerNoTFO)
}

func testRawConnControl(t *testing.T, sc syscall.Conn) {
	rawConn, err := sc.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}

	var success bool

	if err = rawConn.Control(func(fd uintptr) {
		success = fd != 0
	}); err != nil {
		t.Fatal(err)
	}

	if !success {
		t.Error("RawConn Control failed")
	}
}

func testListenCtrlFn(t *testing.T, lc ListenConfig) {
	var success bool

	lc.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			success = fd != 0
		})
	}

	ln, err := lc.Listen(context.Background(), "tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	if !success {
		t.Error("ListenConfig ctrlFn failed")
	}

	testRawConnControl(t, ln.(syscall.Conn))
}

func testDialCtrlFn(t *testing.T, d Dialer) {
	var success bool

	d.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			success = fd != 0
		})
	}

	c, err := d.Dial("tcp", "example.com:443", hello)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if !success {
		t.Error("Dialer ctrlFn failed")
	}

	testRawConnControl(t, c.(syscall.Conn))
}

const (
	ctxKey = 64
	ctxVal = 128
)

func testDialCtrlCtxFn(t *testing.T, d Dialer) {
	var success bool

	d.ControlContext = func(ctx context.Context, network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			success = fd != 0 && ctx.Value(ctxKey) == ctxVal
		})
	}

	ctx := context.WithValue(context.Background(), ctxKey, ctxVal)
	c, err := d.DialContext(ctx, "tcp", "example.com:443", hello)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if !success {
		t.Error("Dialer ctrlCtxFn failed")
	}

	testRawConnControl(t, c.(syscall.Conn))
}

func testDialCtrlCtxFnSupersedesCtrlFn(t *testing.T, d Dialer) {
	var ctrlCount int

	d.Control = func(network, address string, c syscall.RawConn) error {
		ctrlCount++
		return nil
	}

	d.ControlContext = func(ctx context.Context, network, address string, c syscall.RawConn) error {
		ctrlCount++
		return nil
	}

	c, err := d.Dial("tcp", "example.com:443", hello)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if ctrlCount != 1 {
		t.Errorf("Dialer control function called %d times, expected 1", ctrlCount)
	}
}

func testAddrFunctions(t *testing.T, lc ListenConfig, d Dialer) {
	ln, err := lc.Listen(context.Background(), "tcp", "[::1]:")
	if err != nil {
		t.Fatal(err)
	}
	lntcp := ln.(*net.TCPListener)
	defer lntcp.Close()

	addr := lntcp.Addr().(*net.TCPAddr)
	if !addr.IP.Equal(net.IPv6loopback) {
		t.Fatalf("expected unspecified IP, got %v", addr.IP)
	}
	if addr.Port == 0 {
		t.Fatalf("expected non-zero port, got %d", addr.Port)
	}

	c, err := d.Dial("tcp", addr.String(), hello)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if laddr := c.LocalAddr().(*net.TCPAddr); !laddr.IP.Equal(net.IPv6loopback) || laddr.Port == 0 {
		t.Errorf("Bad local addr: %v", laddr)
	}
	if raddr := c.RemoteAddr().(*net.TCPAddr); !raddr.IP.Equal(net.IPv6loopback) || raddr.Port != addr.Port {
		t.Errorf("Bad remote addr: %v", raddr)
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
		t.Errorf("Wrote %d bytes, should have written %d bytes", n, dataLen)
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
		t.Errorf("Wrote %d bytes, should have written %d bytes", bytesWritten, dataLen)
	}
}

func readExactlyOneByte(r io.Reader, expectedByte byte, t *testing.T) {
	b := make([]byte, 1)
	n, err := r.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("Read %d bytes, expected 1 byte", n)
	}
	if b[0] != expectedByte {
		t.Fatalf("Read unexpected byte: '%c', expected '%c'", b[0], expectedByte)
	}
}

func readUntilEOF(r io.Reader, expectedData []byte, t *testing.T) {
	b, err := io.ReadAll(r)
	if err != nil {
		t.Error(err)
		return
	}
	if !bytes.Equal(b, expectedData) {
		t.Errorf("Read data %v is different from original data %v", b, expectedData)
	}
}

func testClientWriteReadServerReadWrite(t *testing.T, lc ListenConfig, d Dialer) {
	t.Logf("c->s payload: %v", helloworld)
	t.Logf("s->c payload: %v", worldhello)

	ln, err := lc.Listen(context.Background(), "tcp", "[::1]:")
	if err != nil {
		t.Fatal(err)
	}
	lntcp := ln.(*net.TCPListener)
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
		ctrlCh <- struct{}{}
	}()

	c, err := d.Dial("tcp", ln.Addr().String(), hello)
	if err != nil {
		t.Fatal(err)
	}
	tc := c.(*net.TCPConn)
	defer tc.Close()

	write(tc, world, t)
	tc.CloseWrite()
	readUntilEOF(tc, worldhello, t)
	<-ctrlCh
}

func testServerWriteReadClientReadWrite(t *testing.T, lc ListenConfig, d Dialer) {
	t.Logf("c->s payload: %v", helloworld)
	t.Logf("s->c payload: %v", worldhello)

	ln, err := lc.Listen(context.Background(), "tcp", "[::1]:")
	if err != nil {
		t.Fatal(err)
	}
	lntcp := ln.(*net.TCPListener)
	defer lntcp.Close()
	t.Log("Started listener on", lntcp.Addr())

	ctrlCh := make(chan struct{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			t.Error(err)
			return
		}
		t.Log("Accepted", conn.RemoteAddr())
		defer conn.Close()

		write(conn, world, t)
		write(conn, hello, t)
		conn.CloseWrite()
		readUntilEOF(conn, helloworld, t)
		ctrlCh <- struct{}{}
	}()

	c, err := d.Dial("tcp", ln.Addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	tc := c.(*net.TCPConn)
	defer tc.Close()

	readUntilEOF(tc, worldhello, t)
	write(tc, hello, t)
	write(tc, world, t)
	tc.CloseWrite()
	<-ctrlCh
}

func testClientServerReadFrom(t *testing.T, lc ListenConfig, d Dialer) {
	t.Logf("c->s payload: %v", helloworld)
	t.Logf("s->c payload: %v", worldhello)

	ln, err := lc.Listen(context.Background(), "tcp", "[::1]:")
	if err != nil {
		t.Fatal(err)
	}
	lntcp := ln.(*net.TCPListener)
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
		writeWithReadFrom(conn, world, t)
		writeWithReadFrom(conn, hello, t)
		conn.CloseWrite()
		ctrlCh <- struct{}{}
	}()

	c, err := d.Dial("tcp", ln.Addr().String(), hello)
	if err != nil {
		t.Fatal(err)
	}
	tc := c.(*net.TCPConn)
	defer tc.Close()

	writeWithReadFrom(tc, world, t)
	tc.CloseWrite()
	readUntilEOF(tc, worldhello, t)
	<-ctrlCh
}

func testSetDeadline(t *testing.T, lc ListenConfig, d Dialer) {
	t.Logf("payload: %v", helloWorldSentence)

	ln, err := lc.Listen(context.Background(), "tcp", "[::1]:")
	if err != nil {
		t.Fatal(err)
	}
	lntcp := ln.(*net.TCPListener)
	defer lntcp.Close()
	t.Log("Started listener on", lntcp.Addr())

	ctrlCh := make(chan struct{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			t.Error(err)
			return
		}
		t.Log("Accepted", conn.RemoteAddr())
		defer conn.Close()

		write(conn, helloWorldSentence, t)
		readUntilEOF(conn, []byte{'h', 'l', 'l', ','}, t)
		ctrlCh <- struct{}{}
	}()

	c, err := d.Dial("tcp", ln.Addr().String(), helloWorldSentence[:1])
	if err != nil {
		t.Fatal(err)
	}
	tc := c.(*net.TCPConn)
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
