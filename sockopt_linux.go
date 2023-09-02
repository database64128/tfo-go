package tfo

import "golang.org/x/sys/unix"

// TCPFastopenQueueLength is the maximum number of total pending TFO connection requests,
// see https://datatracker.ietf.org/doc/html/rfc7413#section-5.1 for why this limit exists.
// The current value aligns with Go std's listen(2) backlog (4096, as of the current version).
//
// Deprecated: This constant is no longer used in this module and will be removed in v3.
const TCPFastopenQueueLength = 4096

func setTFOListener(fd uintptr) error {
	return setTFOListenerWithBacklog(fd, defaultBacklog)
}

func setTFOListenerWithBacklog(fd uintptr, backlog int) error {
	return setTFO(int(fd), backlog)
}

func setTFODialer(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN_CONNECT, 1)
}
