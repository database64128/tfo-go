//go:build darwin || freebsd || linux || windows
// +build darwin freebsd linux windows

package tfo

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"testing"
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
	bytesWritten := 0
	writeBuf := data
	for len(writeBuf) > 0 {
		n, err := w.Write(writeBuf)
		if err != nil {
			t.Error(err)
		}
		bytesWritten += n
		writeBuf = writeBuf[n:]
	}
	if bytesWritten != dataLen {
		t.Error(fmt.Errorf("written %d bytes, should have written %d bytes", bytesWritten, dataLen))
	}
}

func readOnce(r io.Reader, b []byte, expectedData []byte, t *testing.T) {
	dataLen := len(expectedData)
	n, err := r.Read(b)
	if err != nil {
		t.Error(err)
	}
	if n != dataLen {
		t.Error(fmt.Errorf("read %d bytes at once, expected %d bytes", n, dataLen))
	}
	if !bytes.Equal(expectedData, b[:n]) {
		t.Error(fmt.Errorf("read data does not equal original data"))
	}
}

func TestClientWriteServerRead(t *testing.T) {
	data := []byte{'h', 'e', 'l', 'l', 'o'}
	t.Log("payload: hello")

	lntcp, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	fromClientErrCh := make(chan error)
	go func() {
		conn, err := lntcp.Accept()
		if err != nil {
			fromClientErrCh <- err
			return
		}
		defer conn.Close()
		t.Log("accepted", conn.RemoteAddr())

		b := make([]byte, 16)
		readOnce(conn, b, data, t)

		fromClientErrCh <- err
	}()

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv6loopback,
		Port: lntcp.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	write(tc, data, t)

	<-fromClientErrCh
}

func TestClientWriteServerReadWithLocalAddr(t *testing.T) {
	data := []byte{'h', 'e', 'l', 'l', 'o'}
	t.Log("payload: hello")

	lntcp, err := ListenTCP("tcp", &net.TCPAddr{
		IP: net.IPv6loopback,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	fromClientErrCh := make(chan error)
	go func() {
		conn, err := lntcp.Accept()
		if err != nil {
			fromClientErrCh <- err
			return
		}
		defer conn.Close()
		t.Log("accepted", conn.RemoteAddr())

		b := make([]byte, 16)
		readOnce(conn, b, data, t)

		fromClientErrCh <- err
	}()

	tc, err := DialTCP("tcp", &net.TCPAddr{
		IP: net.IPv6loopback,
	}, &net.TCPAddr{
		IP:   net.IPv6loopback,
		Port: lntcp.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	write(tc, data, t)

	<-fromClientErrCh
}

func TestServerWriteClientRead(t *testing.T) {
	data := []byte{'h', 'e', 'l', 'l', 'o'}
	t.Log("payload: hello")

	lntcp, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	fromClientErrCh := make(chan error)
	go func() {
		conn, err := lntcp.Accept()
		if err != nil {
			fromClientErrCh <- err
			return
		}
		defer conn.Close()
		t.Log("accepted", conn.RemoteAddr())

		write(conn, data, t)

		fromClientErrCh <- err
	}()

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv6loopback,
		Port: lntcp.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	b := make([]byte, 16)
	readOnce(tc, b, data, t)

	<-fromClientErrCh
}

type fakeReader struct {
	data []byte
	pos  int
}

func (r *fakeReader) Read(b []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(b, r.data[n:])
	r.pos += n
	return
}

func writeWithReadFrom(w io.ReaderFrom, data []byte, t *testing.T) {
	dataLen := len(data)
	r := &fakeReader{
		data: data,
	}
	n, err := w.ReadFrom(r)
	if err != nil {
		t.Error(err)
	}
	bytesWritten := int(n)
	if bytesWritten != dataLen {
		t.Error(fmt.Errorf("written %d bytes, should have written %d bytes", bytesWritten, dataLen))
	}
}

func TestFakeReader(t *testing.T) {
	data := []byte{'h', 'e', 'l', 'l', 'o'}
	t.Log("payload: hello")
	writeWithReadFrom(io.Discard.(io.ReaderFrom), data, t)
}

func TestClientWriteWithReadFromServerRead(t *testing.T) {
	data := []byte{'h', 'e', 'l', 'l', 'o'}
	t.Log("payload: hello")

	lntcp, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	fromClientErrCh := make(chan error)
	go func() {
		conn, err := lntcp.Accept()
		if err != nil {
			fromClientErrCh <- err
			return
		}
		defer conn.Close()
		t.Log("accepted", conn.RemoteAddr())

		b := make([]byte, 16)
		readOnce(conn, b, data, t)

		fromClientErrCh <- err
	}()

	tc, err := DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv6loopback,
		Port: lntcp.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	writeWithReadFrom(tc, data, t)

	<-fromClientErrCh
}
