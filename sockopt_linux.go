package tfo

import (
	"os"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"
)

// TCPFastopenQueueLength is the maximum number of total pending TFO connection requests,
// see https://datatracker.ietf.org/doc/html/rfc7413#section-5.1 for why this limit exists.
// The current value is the default net.core.somaxconn on Linux.
//
// Deprecated: This constant is no longer used in this module and will be removed in v3.
const TCPFastopenQueueLength = 4096

func setTFOListener(fd uintptr) error {
	return setTFOListenerWithBacklog(fd, 0)
}

func setTFOListenerWithBacklog(fd uintptr, backlog int) error {
	if backlog == 0 {
		backlog = maxListenerBacklog()
	}
	return setTFO(int(fd), backlog)
}

// maxListenerBacklog returns a cached value of net.core.somaxconn, which is the maximum
// listen(2) backlog on Linux. The kernel clamps the TFO backlog to this value too.
var maxListenerBacklog = sync.OnceValue(func() int {
	// Simplified from src/net/sock_linux.go
	b, err := os.ReadFile("/proc/sys/net/core/somaxconn")
	if err != nil {
		return unix.SOMAXCONN
	}

	backlog, err := strconv.Atoi(string(b))
	if err != nil {
		return unix.SOMAXCONN
	}
	return backlog
})

func setTFODialer(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN_CONNECT, 1)
}
