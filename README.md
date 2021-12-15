# `tfo-go`

[![Go Reference](https://pkg.go.dev/badge/github.com/database64128/tfo-go.svg)](https://pkg.go.dev/github.com/database64128/tfo-go)
[![Test](https://github.com/database64128/tfo-go/actions/workflows/test.yml/badge.svg)](https://github.com/database64128/tfo-go/actions/workflows/test.yml)

`tfo-go` provides a series of wrappers around `net.ListenConfig`, `net.Listen()`, `net.ListenTCP()`, `net.Dialer`, `net.Dial()`, `net.DialTCP()` that seamlessly enable TCP Fast Open. These wrapper types and functions can be used as drop-in replacements for their counterparts in Go `net` with minimal changes required.

`tfo-go` supports Linux, Windows, macOS, and FreeBSD. On unsupported platforms, `tfo-go` automatically falls back to non-TFO sockets and returns `ErrPlatformUnsupported`. Make sure to check and handle/ignore such errors in your code.

## ⚠️ Limitations

1. On Windows, all operations on a TFO-enabled connection will block the current goroutine thread, because there's no way for `tfo-go` to utilize Go's runtime poller on Windows. For real world applications with a fairly low number of connections, `tfo-go` will work just fine. If your application needs to handle a lot of concurrent I/O, just don't use Windows!
2. FreeBSD code is completely untested. Use at your own risk. Feedback is welcome.
