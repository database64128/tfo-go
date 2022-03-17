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
		bytesWritten += n
		if err != nil {
			t.Error(err)
			break
		}
		writeBuf = writeBuf[n:]
	}
	if bytesWritten != dataLen {
		t.Errorf("written %d bytes, should have written %d bytes", bytesWritten, dataLen)
	}
}

func writeOneByteAtATime(w io.Writer, data []byte, t *testing.T) {
	dataLen := len(data)
	bytesWritten := 0
	writeBuf := data
	for len(writeBuf) > 0 {
		n, err := w.Write(writeBuf[:1])
		bytesWritten += n
		if err != nil {
			t.Error(err)
			break
		}
		writeBuf = writeBuf[n:]
	}
	if bytesWritten != dataLen {
		t.Errorf("written %d bytes, should have written %d bytes", bytesWritten, dataLen)
	}
}

func readOnce(r io.Reader, b []byte, expectedData []byte, t *testing.T) {
	dataLen := len(expectedData)
	n, err := r.Read(b)
	if err != nil {
		t.Error(err)
	}
	if n != dataLen {
		t.Errorf("read %d bytes at once, expected %d bytes", n, dataLen)
	}
	if !bytes.Equal(expectedData, b[:n]) {
		t.Errorf("read data does not equal original data")
	}
}

func readExactlyOneByte(r io.Reader, b []byte, expectedByte byte, t *testing.T) {
	n, err := r.Read(b[:1])
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

func readUntilEOF(r io.Reader, b []byte, expectedData []byte, t *testing.T) {
	dataLen := len(expectedData)
	bytesRead := 0
	readBuf := b
	for {
		n, err := r.Read(readBuf)
		bytesRead += n
		if err != nil && err != io.EOF {
			t.Error(err)
			break
		}
		if n == 0 || err == io.EOF {
			break
		}
		readBuf = readBuf[n:]
	}
	if bytesRead != dataLen {
		t.Errorf("read %d bytes at once, expected %d bytes", bytesRead, dataLen)
	}
	if !bytes.Equal(expectedData, b[:bytesRead]) {
		t.Errorf("read data does not equal original data")
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

func TestClientWriteServerReadv4(t *testing.T) {
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

	tc, err := net.DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: lntcp.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()

	write(tc, data, t)

	<-fromClientErrCh
}

func TestClientWriteServerReadWithContext(t *testing.T) {
	data := []byte{'h', 'e', 'l', 'l', 'o'}
	t.Log("payload: hello")

	var lc ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	t.Log("listening on", ln.Addr())

	fromClientErrCh := make(chan error)
	go func() {
		conn, err := ln.Accept()
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

	var dialer Dialer
	c, err := dialer.Dial("tcp", fmt.Sprintf("localhost:%d", ln.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	write(c, data, t)

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

func TestClientMultipleWritesServerReadAll(t *testing.T) {
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
		readUntilEOF(conn, b, data, t)

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

	writeOneByteAtATime(tc, data, t)
	tc.CloseWrite()

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
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			fromClientErrCh <- err
			return
		}
		t.Log("accepted", conn.RemoteAddr())
		defer conn.Close()

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
		t.Errorf("written %d bytes, should have written %d bytes", bytesWritten, dataLen)
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

func TestSetDeadline(t *testing.T) {
	data := []byte{'h', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd', '!', '\n'}
	t.Log("payload: hello, world!")

	lntcp, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lntcp.Close()
	t.Log("listening on", lntcp.Addr())

	fromClientErrCh := make(chan error)
	safe2close := make(chan interface{})
	go func() {
		conn, err := lntcp.AcceptTCP()
		if err != nil {
			fromClientErrCh <- err
			return
		}
		t.Log("accepted", conn.RemoteAddr())
		defer conn.Close()

		write(conn, data, t)

		<-safe2close
		b := make([]byte, 16)
		readUntilEOF(conn, b, []byte{'h', 'l', 'l', ','}, t)
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

	b := make([]byte, 1)

	// SetReadDeadline
	readExactlyOneByte(tc, b, 'h', t)
	if err := tc.SetReadDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := tc.Read(b); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(n, err)
	}
	if err := tc.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	readExactlyOneByte(tc, b, 'e', t)

	// SetWriteDeadline
	write(tc, data[:1], t)
	if err := tc.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := tc.Write(data[1:2]); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(n, err)
	}
	if err := tc.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	write(tc, data[2:3], t)

	// SetDeadline
	readExactlyOneByte(tc, b, 'l', t)
	write(tc, data[3:4], t)
	if err := tc.SetDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := tc.Read(b); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(err)
	}
	if n, err := tc.Write(data[4:5]); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(n, err)
	}
	if err := tc.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	readExactlyOneByte(tc, b, 'l', t)
	write(tc, data[5:6], t)

	tc.CloseWrite()
	safe2close <- nil
	<-fromClientErrCh
}
