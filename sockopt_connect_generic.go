//go:build freebsd || (windows && (!go1.23 || (go1.23 && tfogo_checklinkname0)))

package tfo

func setTFODialer(fd uintptr) error {
	return setTFO(int(fd), 1)
}
