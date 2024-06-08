//go:build !darwin && !freebsd && !linux && (!windows || (windows && go1.23 && !tfogo_checklinkname0))

package tfo

func setTFODialer(_ uintptr) error {
	return ErrPlatformUnsupported
}
