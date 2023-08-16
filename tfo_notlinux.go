//go:build !linux

package tfo

// SetTFOListenerWithBacklog enables TCP Fast Open on the listener with the given backlog.
// If the platform does not support custom backlog, the specified backlog is ignored.
func SetTFOListenerWithBacklog(fd uintptr, backlog int) error {
	return SetTFOListener(fd)
}
