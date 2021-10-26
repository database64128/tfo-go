# `tfo-go`

`tfo-go` provides a series of wrappers around `net.Listen`, `net.ListenTCP`, `net.DialContext`, `net.Dial`, `net.DialTCP` that seamlessly enable TCP Fast Open. These wrapper functions can be used as drop-in replacements for Go `net`'s functions with minimal changes required.

`tfo-go` supports Linux, Windows, macOS, FreeBSD. On unsupported platforms, `tfo-go` automatically falls back to non-TFO sockets and returns `ErrPlatformUnsupported`. Make sure to check and handle/ignore such errors in your code.

## Listen

```go
func ListenContext(ctx context.Context, network, address string) (net.Listener, error)
func Listen(network, address string) (net.Listener, error)
func ListenTCP(network string, laddr *net.TCPAddr) (*net.TCPListener, error)
```

## Dial

```go
func DialContext(ctx context.Context, network, address string) (net.Conn, error)
func Dial(network, address string) (net.Conn, error)
func DialTCP(network string, laddr, raddr *net.TCPAddr) (*net.TCPConn, error)
```

## Control

```go
func SetTFOListener(fd uintptr) error
func SetTFODialer(fd uintptr) error
```
