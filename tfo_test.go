package tfo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

type mptcpStatus uint8

const (
	mptcpUseDefault mptcpStatus = iota
	mptcpEnabled
	mptcpDisabled
)

var listenConfigCases = []struct {
	name         string
	listenConfig ListenConfig
	mptcp        mptcpStatus
}{
	{"TFO", ListenConfig{}, mptcpUseDefault},
	{"TFO+MPTCPEnabled", ListenConfig{}, mptcpEnabled},
	{"TFO+MPTCPDisabled", ListenConfig{}, mptcpDisabled},
	{"TFO+Backlog1024", ListenConfig{Backlog: 1024}, mptcpUseDefault},
	{"TFO+Backlog1024+MPTCPEnabled", ListenConfig{Backlog: 1024}, mptcpEnabled},
	{"TFO+Backlog1024+MPTCPDisabled", ListenConfig{Backlog: 1024}, mptcpDisabled},
	{"TFO+Backlog-1", ListenConfig{Backlog: -1}, mptcpUseDefault},
	{"TFO+Backlog-1+MPTCPEnabled", ListenConfig{Backlog: -1}, mptcpEnabled},
	{"TFO+Backlog-1+MPTCPDisabled", ListenConfig{Backlog: -1}, mptcpDisabled},
	{"TFO+Fallback", ListenConfig{Fallback: true}, mptcpUseDefault},
	{"TFO+Fallback+MPTCPEnabled", ListenConfig{Fallback: true}, mptcpEnabled},
	{"TFO+Fallback+MPTCPDisabled", ListenConfig{Fallback: true}, mptcpDisabled},
	{"NoTFO", ListenConfig{DisableTFO: true}, mptcpUseDefault},
	{"NoTFO+MPTCPEnabled", ListenConfig{DisableTFO: true}, mptcpEnabled},
	{"NoTFO+MPTCPDisabled", ListenConfig{DisableTFO: true}, mptcpDisabled},
	{"NoTFO+Backlog1024", ListenConfig{DisableTFO: true, Backlog: 1024}, mptcpUseDefault},
	{"NoTFO+Backlog1024+MPTCPEnabled", ListenConfig{DisableTFO: true, Backlog: 1024}, mptcpEnabled},
	{"NoTFO+Backlog1024+MPTCPDisabled", ListenConfig{DisableTFO: true, Backlog: 1024}, mptcpDisabled},
	{"NoTFO+Backlog-1", ListenConfig{DisableTFO: true, Backlog: -1}, mptcpUseDefault},
	{"NoTFO+Backlog-1+MPTCPEnabled", ListenConfig{DisableTFO: true, Backlog: -1}, mptcpEnabled},
	{"NoTFO+Backlog-1+MPTCPDisabled", ListenConfig{DisableTFO: true, Backlog: -1}, mptcpDisabled},
	{"NoTFO+Fallback", ListenConfig{DisableTFO: true, Fallback: true}, mptcpUseDefault},
	{"NoTFO+Fallback+MPTCPEnabled", ListenConfig{DisableTFO: true, Fallback: true}, mptcpEnabled},
	{"NoTFO+Fallback+MPTCPDisabled", ListenConfig{DisableTFO: true, Fallback: true}, mptcpDisabled},
}

var dialerCases = []struct {
	name   string
	dialer Dialer
	mptcp  mptcpStatus
}{
	{"TFO", Dialer{}, mptcpUseDefault},
	{"TFO+MPTCPEnabled", Dialer{}, mptcpEnabled},
	{"TFO+MPTCPDisabled", Dialer{}, mptcpDisabled},
	{"TFO+Fallback", Dialer{Fallback: true}, mptcpUseDefault},
	{"TFO+Fallback+MPTCPEnabled", Dialer{Fallback: true}, mptcpEnabled},
	{"TFO+Fallback+MPTCPDisabled", Dialer{Fallback: true}, mptcpDisabled},
	{"NoTFO", Dialer{DisableTFO: true}, mptcpUseDefault},
	{"NoTFO+MPTCPEnabled", Dialer{DisableTFO: true}, mptcpEnabled},
	{"NoTFO+MPTCPDisabled", Dialer{DisableTFO: true}, mptcpDisabled},
	{"NoTFO+Fallback", Dialer{DisableTFO: true, Fallback: true}, mptcpUseDefault},
	{"NoTFO+Fallback+MPTCPEnabled", Dialer{DisableTFO: true, Fallback: true}, mptcpEnabled},
	{"NoTFO+Fallback+MPTCPDisabled", Dialer{DisableTFO: true, Fallback: true}, mptcpDisabled},
}

type testCase struct {
	name         string
	listenConfig ListenConfig
	dialer       Dialer
}

// cases is a list of [ListenConfig] and [Dialer] combinations to test.
var cases []testCase

func init() {
	// Initialize [listenConfigCases].
	for i := range listenConfigCases {
		c := &listenConfigCases[i]
		switch c.mptcp {
		case mptcpUseDefault:
		case mptcpEnabled:
			c.listenConfig.SetMultipathTCP(true)
		case mptcpDisabled:
			c.listenConfig.SetMultipathTCP(false)
		default:
			panic("unreachable")
		}
	}

	// Initialize [dialerCases].
	for i := range dialerCases {
		c := &dialerCases[i]
		switch c.mptcp {
		case mptcpUseDefault:
		case mptcpEnabled:
			c.dialer.SetMultipathTCP(true)
		case mptcpDisabled:
			c.dialer.SetMultipathTCP(false)
		default:
			panic("unreachable")
		}
	}

	// Generate [cases].
	cases = make([]testCase, 0, len(listenConfigCases)*len(dialerCases))
	for _, lc := range listenConfigCases {
		if comptimeNoTFO && !lc.listenConfig.tfoDisabled() {
			continue
		}
		for _, d := range dialerCases {
			if comptimeNoTFO && !d.dialer.DisableTFO {
				continue
			}
			cases = append(cases, testCase{
				name:         lc.name + "/" + d.name,
				listenConfig: lc.listenConfig,
				dialer:       d.dialer,
			})
		}
	}
}

// discardTCPServer is a TCP server that accepts and drains incoming connections.
type discardTCPServer struct {
	ln *net.TCPListener
	wg sync.WaitGroup
}

// newDiscardTCPServer creates a new [discardTCPServer] that listens on a random port.
func newDiscardTCPServer(ctx context.Context) (*discardTCPServer, error) {
	lc := ListenConfig{DisableTFO: comptimeNoTFO}
	ln, err := lc.Listen(ctx, "tcp", "[::1]:")
	if err != nil {
		return nil, err
	}
	return &discardTCPServer{ln: ln.(*net.TCPListener)}, nil
}

// Addr returns the server's address.
func (s *discardTCPServer) Addr() *net.TCPAddr {
	return s.ln.Addr().(*net.TCPAddr)
}

// Start spins up a new goroutine that accepts and drains incoming connections
// until [discardTCPServer.Close] is called.
func (s *discardTCPServer) Start(t *testing.T) {
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()

		for {
			c, err := s.ln.AcceptTCP()
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					return
				}
				t.Error("AcceptTCP:", err)
				return
			}

			go func() {
				defer c.Close()

				n, err := io.Copy(io.Discard, c)
				if err != nil {
					t.Error("Copy:", err)
				}
				t.Logf("Discarded %d bytes from %s", n, c.RemoteAddr())
			}()
		}
	}()
}

// Close interrupts all running accept goroutines, waits for them to finish,
// and closes the listener.
func (s *discardTCPServer) Close() {
	s.ln.SetDeadline(aLongTimeAgo)
	s.wg.Wait()
	s.ln.Close()
}

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
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testListenDialUDP(t, c.listenConfig, c.dialer)
		})
	}
}

// TestListenCtrlFn ensures that the user-provided [ListenConfig.Control] function
// is called when [ListenConfig.Listen] is called.
func TestListenCtrlFn(t *testing.T) {
	for _, c := range listenConfigCases {
		t.Run(c.name, func(t *testing.T) {
			testListenCtrlFn(t, c.listenConfig)
		})
	}
}

// TestDialCtrlFn ensures that [Dialer]'s user-provided control functions
// are used in the same way as [net.Dialer].
func TestDialCtrlFn(t *testing.T) {
	s, err := newDiscardTCPServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	address := s.Addr().String()

	for _, c := range dialerCases {
		t.Run(c.name, func(t *testing.T) {
			testDialCtrlFn(t, c.dialer, address)
			testDialCtrlCtxFn(t, c.dialer, address)
			testDialCtrlCtxFnSupersedesCtrlFn(t, c.dialer, address)
		})
	}
}

// TestAddrFunctions ensures that the address methods on [*net.TCPListener] and
// [*net.TCPConn] return the correct values.
func TestAddrFunctions(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testAddrFunctions(t, c.listenConfig, c.dialer)
		})
	}
}

// TestClientWriteReadServerReadWrite ensures that a client can write to a server,
// the server can read from the client, and the server can write to the client.
func TestClientWriteReadServerReadWrite(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testClientWriteReadServerReadWrite(t, c.listenConfig, c.dialer)
		})
	}
}

// TestServerWriteReadClientReadWrite ensures that a server can write to a client,
// the client can read from the server, and the client can write to the server.
func TestServerWriteReadClientReadWrite(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testServerWriteReadClientReadWrite(t, c.listenConfig, c.dialer)
		})
	}
}

// TestClientServerReadFrom ensures that the ReadFrom method
// on accepted and dialed connections works as expected.
func TestClientServerReadFrom(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testClientServerReadFrom(t, c.listenConfig, c.dialer)
		})
	}
}

// TestSetDeadline ensures that the SetDeadline, SetReadDeadline, and
// SetWriteDeadline methods on accepted and dialed connections work as expected.
func TestSetDeadline(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testSetDeadline(t, c.listenConfig, c.dialer)
		})
	}
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

func testDialCtrlFn(t *testing.T, d Dialer, address string) {
	var success bool

	d.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			success = fd != 0
		})
	}

	c, err := d.Dial("tcp", address, hello)
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

func testDialCtrlCtxFn(t *testing.T, d Dialer, address string) {
	var success bool

	d.ControlContext = func(ctx context.Context, network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			success = fd != 0 && ctx.Value(ctxKey) == ctxVal
		})
	}

	ctx := context.WithValue(context.Background(), ctxKey, ctxVal)
	c, err := d.DialContext(ctx, "tcp", address, hello)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if !success {
		t.Error("Dialer ctrlCtxFn failed")
	}

	testRawConnControl(t, c.(syscall.Conn))
}

func testDialCtrlCtxFnSupersedesCtrlFn(t *testing.T, d Dialer, address string) {
	var ctrlCtxFnCalled bool

	d.Control = func(network, address string, c syscall.RawConn) error {
		t.Error("Dialer.Control called")
		return nil
	}

	d.ControlContext = func(ctx context.Context, network, address string, c syscall.RawConn) error {
		ctrlCtxFnCalled = true
		return nil
	}

	c, err := d.Dial("tcp", address, hello)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if !ctrlCtxFnCalled {
		t.Error("Dialer.ControlContext not called")
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
		close(ctrlCh)
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
		close(ctrlCh)
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
		close(ctrlCh)
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
		close(ctrlCh)
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
