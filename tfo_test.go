package tfo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
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

type runtimeFallbackHelperFunc func(*testing.T)

func runtimeFallbackAsIs(t *testing.T) {}

func runtimeFallbackSetListenNoTFO(t *testing.T) {
	if runtimeListenNoTFO.CompareAndSwap(false, true) {
		t.Cleanup(func() {
			runtimeListenNoTFO.Store(false)
		})
	}
}

func runtimeFallbackSetDialNoTFO(t *testing.T) {
	if v := runtimeDialTFOSupport.v.Swap(uint32(dialTFOSupportNone)); v != uint32(dialTFOSupportNone) {
		t.Cleanup(func() {
			runtimeDialTFOSupport.v.Store(v)
		})
	}
}

func runtimeFallbackSetDialLinuxSendto(t *testing.T) {
	if v := runtimeDialTFOSupport.v.Swap(uint32(dialTFOSupportLinuxSendto)); v != uint32(dialTFOSupportLinuxSendto) {
		t.Cleanup(func() {
			runtimeDialTFOSupport.v.Store(v)
		})
	}
}

type listenConfigTestCase struct {
	name               string
	listenConfig       ListenConfig
	mptcp              mptcpStatus
	wantTFO            bool
	setRuntimeFallback runtimeFallbackHelperFunc
}

func (c listenConfigTestCase) shouldSkip() bool {
	return comptimeListenNoTFO && !c.listenConfig.tfoDisabled()
}

func (c listenConfigTestCase) checkSkip(t *testing.T) {
	if c.shouldSkip() {
		t.Skip("not applicable to the current platform")
	}
}

var listenConfigCases = []listenConfigTestCase{
	{
		name:               "TFO",
		listenConfig:       ListenConfig{},
		mptcp:              mptcpUseDefault,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+RuntimeNoTFO",
		listenConfig:       ListenConfig{},
		mptcp:              mptcpUseDefault,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackSetListenNoTFO,
	},
	{
		name:               "TFO+MPTCPEnabled",
		listenConfig:       ListenConfig{},
		mptcp:              mptcpEnabled,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+MPTCPEnabled+RuntimeNoTFO",
		listenConfig:       ListenConfig{},
		mptcp:              mptcpEnabled,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackSetListenNoTFO,
	},
	{
		name:               "TFO+MPTCPDisabled",
		listenConfig:       ListenConfig{},
		mptcp:              mptcpDisabled,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+MPTCPDisabled+RuntimeNoTFO",
		listenConfig:       ListenConfig{},
		mptcp:              mptcpDisabled,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackSetListenNoTFO,
	},
	{
		name:               "TFO+Backlog1024",
		listenConfig:       ListenConfig{Backlog: 1024},
		mptcp:              mptcpUseDefault,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Backlog1024+MPTCPEnabled",
		listenConfig:       ListenConfig{Backlog: 1024},
		mptcp:              mptcpEnabled,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Backlog1024+MPTCPDisabled",
		listenConfig:       ListenConfig{Backlog: 1024},
		mptcp:              mptcpDisabled,
		wantTFO:            true,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Backlog-1",
		listenConfig:       ListenConfig{Backlog: -1},
		mptcp:              mptcpUseDefault,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Backlog-1+MPTCPEnabled",
		listenConfig:       ListenConfig{Backlog: -1},
		mptcp:              mptcpEnabled,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Backlog-1+MPTCPDisabled",
		listenConfig:       ListenConfig{Backlog: -1},
		mptcp:              mptcpDisabled,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Fallback",
		listenConfig:       ListenConfig{Fallback: true},
		mptcp:              mptcpUseDefault,
		wantTFO:            !comptimeListenNoTFO,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Fallback+RuntimeNoTFO",
		listenConfig:       ListenConfig{Fallback: true},
		mptcp:              mptcpUseDefault,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackSetListenNoTFO,
	},
	{
		name:               "TFO+Fallback+MPTCPEnabled",
		listenConfig:       ListenConfig{Fallback: true},
		mptcp:              mptcpEnabled,
		wantTFO:            !comptimeListenNoTFO,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Fallback+MPTCPEnabled+RuntimeNoTFO",
		listenConfig:       ListenConfig{Fallback: true},
		mptcp:              mptcpEnabled,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackSetListenNoTFO,
	},
	{
		name:               "TFO+Fallback+MPTCPDisabled",
		listenConfig:       ListenConfig{Fallback: true},
		mptcp:              mptcpDisabled,
		wantTFO:            !comptimeListenNoTFO,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Fallback+MPTCPDisabled+RuntimeNoTFO",
		listenConfig:       ListenConfig{Fallback: true},
		mptcp:              mptcpDisabled,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackSetListenNoTFO,
	},
	{
		name:               "NoTFO",
		listenConfig:       ListenConfig{DisableTFO: true},
		mptcp:              mptcpUseDefault,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "NoTFO+MPTCPEnabled",
		listenConfig:       ListenConfig{DisableTFO: true},
		mptcp:              mptcpEnabled,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "NoTFO+MPTCPDisabled",
		listenConfig:       ListenConfig{DisableTFO: true},
		mptcp:              mptcpDisabled,
		wantTFO:            false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
}

type dialerTestCase struct {
	name               string
	dialer             Dialer
	mptcp              mptcpStatus
	wantTFO            bool
	linuxOnly          bool
	setRuntimeFallback runtimeFallbackHelperFunc
}

func (c dialerTestCase) shouldSkip() bool {
	if comptimeDialNoTFO && !c.dialer.DisableTFO {
		return true
	}
	switch runtime.GOOS {
	case "linux", "android":
	default:
		if c.linuxOnly {
			return true
		}
	}
	return false
}

func (c dialerTestCase) checkSkip(t *testing.T) {
	if c.shouldSkip() {
		t.Skip("not applicable to the current platform")
	}
}

var dialerCases = []dialerTestCase{
	{
		name:               "TFO",
		dialer:             Dialer{},
		mptcp:              mptcpUseDefault,
		wantTFO:            true,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+RuntimeNoTFO",
		dialer:             Dialer{},
		mptcp:              mptcpUseDefault,
		wantTFO:            true,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackSetDialNoTFO,
	},
	{
		name:               "TFO+RuntimeLinuxSendto",
		dialer:             Dialer{},
		mptcp:              mptcpUseDefault,
		wantTFO:            true,
		linuxOnly:          true,
		setRuntimeFallback: runtimeFallbackSetDialLinuxSendto,
	},
	{
		name:               "TFO+MPTCPEnabled",
		dialer:             Dialer{},
		mptcp:              mptcpEnabled,
		wantTFO:            true,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+MPTCPEnabled+RuntimeNoTFO",
		dialer:             Dialer{},
		mptcp:              mptcpEnabled,
		wantTFO:            true,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackSetDialNoTFO,
	},
	{
		name:               "TFO+MPTCPEnabled+RuntimeLinuxSendto",
		dialer:             Dialer{},
		mptcp:              mptcpEnabled,
		wantTFO:            true,
		linuxOnly:          true,
		setRuntimeFallback: runtimeFallbackSetDialLinuxSendto,
	},
	{
		name:               "TFO+MPTCPDisabled",
		dialer:             Dialer{},
		mptcp:              mptcpDisabled,
		wantTFO:            true,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+MPTCPDisabled+RuntimeNoTFO",
		dialer:             Dialer{},
		mptcp:              mptcpDisabled,
		wantTFO:            true,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackSetDialNoTFO,
	},
	{
		name:               "TFO+MPTCPDisabled+RuntimeLinuxSendto",
		dialer:             Dialer{},
		mptcp:              mptcpDisabled,
		wantTFO:            true,
		linuxOnly:          true,
		setRuntimeFallback: runtimeFallbackSetDialLinuxSendto,
	},
	{
		name:               "TFO+Fallback",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpUseDefault,
		wantTFO:            !comptimeDialNoTFO,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Fallback+RuntimeNoTFO",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpUseDefault,
		wantTFO:            false,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackSetDialNoTFO,
	},
	{
		name:               "TFO+Fallback+RuntimeLinuxSendto",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpUseDefault,
		wantTFO:            !comptimeDialNoTFO,
		linuxOnly:          true,
		setRuntimeFallback: runtimeFallbackSetDialLinuxSendto,
	},
	{
		name:               "TFO+Fallback+MPTCPEnabled",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpEnabled,
		wantTFO:            !comptimeDialNoTFO,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Fallback+MPTCPEnabled+RuntimeNoTFO",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpEnabled,
		wantTFO:            false,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackSetDialNoTFO,
	},
	{
		name:               "TFO+Fallback+MPTCPEnabled+RuntimeLinuxSendto",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpEnabled,
		wantTFO:            !comptimeDialNoTFO,
		linuxOnly:          true,
		setRuntimeFallback: runtimeFallbackSetDialLinuxSendto,
	},
	{
		name:               "TFO+Fallback+MPTCPDisabled",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpDisabled,
		wantTFO:            !comptimeDialNoTFO,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "TFO+Fallback+MPTCPDisabled+RuntimeNoTFO",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpDisabled,
		wantTFO:            false,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackSetDialNoTFO,
	},
	{
		name:               "TFO+Fallback+MPTCPDisabled+RuntimeLinuxSendto",
		dialer:             Dialer{Fallback: true},
		mptcp:              mptcpDisabled,
		wantTFO:            !comptimeDialNoTFO,
		linuxOnly:          true,
		setRuntimeFallback: runtimeFallbackSetDialLinuxSendto,
	},
	{
		name:               "NoTFO",
		dialer:             Dialer{DisableTFO: true},
		mptcp:              mptcpUseDefault,
		wantTFO:            false,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "NoTFO+MPTCPEnabled",
		dialer:             Dialer{DisableTFO: true},
		mptcp:              mptcpEnabled,
		wantTFO:            false,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
	{
		name:               "NoTFO+MPTCPDisabled",
		dialer:             Dialer{DisableTFO: true},
		mptcp:              mptcpDisabled,
		wantTFO:            false,
		linuxOnly:          false,
		setRuntimeFallback: runtimeFallbackAsIs,
	},
}

type testCase struct {
	name                     string
	listenConfig             ListenConfig
	dialer                   Dialer
	setRuntimeFallbackListen runtimeFallbackHelperFunc
	setRuntimeFallbackDial   runtimeFallbackHelperFunc
}

func (c testCase) Run(t *testing.T, f func(*testing.T, ListenConfig, Dialer)) {
	t.Run(c.name, func(t *testing.T) {
		c.setRuntimeFallbackListen(t)
		c.setRuntimeFallbackDial(t)
		f(t, c.listenConfig, c.dialer)
	})
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
		if lc.shouldSkip() {
			continue
		}
		for _, d := range dialerCases {
			if d.shouldSkip() {
				continue
			}
			cases = append(cases, testCase{
				name:                     lc.name + "/" + d.name,
				listenConfig:             lc.listenConfig,
				dialer:                   d.dialer,
				setRuntimeFallbackListen: lc.setRuntimeFallback,
				setRuntimeFallbackDial:   d.setRuntimeFallback,
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
	lc := ListenConfig{DisableTFO: comptimeListenNoTFO}
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
		c.Run(t, testListenDialUDP)
	}
}

// TestListenCtrlFn ensures that the user-provided [ListenConfig.Control] function
// is called when [ListenConfig.Listen] is called.
func TestListenCtrlFn(t *testing.T) {
	for _, c := range listenConfigCases {
		t.Run(c.name, func(t *testing.T) {
			c.checkSkip(t)
			c.setRuntimeFallback(t)
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
			c.checkSkip(t)
			c.setRuntimeFallback(t)
			testDialCtrlFn(t, c.dialer, address)
			testDialCtrlCtxFn(t, c.dialer, address)
			testDialCtrlCtxFnSupersedesCtrlFn(t, c.dialer, address)
		})
	}
}

// TestListenTFOStatus ensures that [ListenConfig.TFO] reports the correct status.
func TestListenTFOStatus(t *testing.T) {
	for _, c := range listenConfigCases {
		t.Run(c.name, func(t *testing.T) {
			c.setRuntimeFallback(t)
			if got := c.listenConfig.TFO(); got != c.wantTFO {
				t.Errorf("c.listenConfig.TFO() = %v, want %v", got, c.wantTFO)
			}
		})
	}
}

// TestDialTFOStatus ensures that [Dialer.TFO] reports the correct status.
func TestDialTFOStatus(t *testing.T) {
	for _, c := range dialerCases {
		t.Run(c.name, func(t *testing.T) {
			c.setRuntimeFallback(t)
			if got := c.dialer.TFO(); got != c.wantTFO {
				t.Errorf("c.dialer.TFO() = %v, want %v", got, c.wantTFO)
			}
		})
	}
}

// TestAddrFunctions ensures that the address methods on [*net.TCPListener] and
// [*net.TCPConn] return the correct values.
func TestAddrFunctions(t *testing.T) {
	for _, c := range cases {
		c.Run(t, testAddrFunctions)
	}
}

// TestClientWriteReadServerReadWrite ensures that a client can write to a server,
// the server can read from the client, and the server can write to the client.
func TestClientWriteReadServerReadWrite(t *testing.T) {
	for _, c := range cases {
		c.Run(t, testClientWriteReadServerReadWrite)
	}
}

// TestServerWriteReadClientReadWrite ensures that a server can write to a client,
// the client can read from the server, and the client can write to the server.
func TestServerWriteReadClientReadWrite(t *testing.T) {
	for _, c := range cases {
		c.Run(t, testServerWriteReadClientReadWrite)
	}
}

// TestClientServerReadFrom ensures that the ReadFrom method
// on accepted and dialed connections works as expected.
func TestClientServerReadFrom(t *testing.T) {
	for _, c := range cases {
		c.Run(t, testClientServerReadFrom)
	}
}

// TestSetDeadline ensures that the SetDeadline, SetReadDeadline, and
// SetWriteDeadline methods on accepted and dialed connections work as expected.
func TestSetDeadline(t *testing.T) {
	for _, c := range cases {
		c.Run(t, testSetDeadline)
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

func testDialCtrlCtxFn(t *testing.T, d Dialer, address string) {
	type contextKey int

	const (
		ctxKey = contextKey(64)
		ctxVal = 128
	)

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
	t.Helper()
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
	t.Helper()
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
	t.Helper()
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
	t.Helper()
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
