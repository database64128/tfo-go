//go:build darwin || freebsd || linux || (windows && tfogo_checklinkname0)

package tfo

import (
	"context"
	"net"
	"net/netip"
	"os"
	"slices"
	"syscall"
	"time"
)

const comptimeDialNoTFO = false

// Boolean to int.
func boolint(b bool) int {
	if b {
		return 1
	}
	return 0
}

// wrapSyscallError takes an error and a syscall name. If the error is
// a syscall.Errno, it wraps it in a os.SyscallError using the syscall name.
func wrapSyscallError(name string, err error) error {
	if _, ok := err.(syscall.Errno); ok {
		err = os.NewSyscallError(name, err)
	}
	return err
}

// Modified from favoriteAddrFamily in src/net/ipsock_posix.go
func favoriteDialAddrFamily(network string, laddr, raddr netip.AddrPort) (family int, ipv6only bool) {
	switch network {
	case "tcp4":
		return syscall.AF_INET, false
	case "tcp6":
		return syscall.AF_INET6, true
	}

	if laddr.Addr().Is4() || laddr.Addr().Is4In6() || raddr.Addr().Is4() || raddr.Addr().Is4In6() {
		return syscall.AF_INET, false
	}
	return syscall.AF_INET6, false
}

func ctrlNetwork(network string, family int) string {
	if network == "tcp4" || family == syscall.AF_INET {
		return "tcp4"
	}
	return "tcp6"
}

func (d *Dialer) dialCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		panic("nil context")
	}
	deadline := d.deadline(ctx, time.Now())
	var cancel1, cancel2 context.CancelFunc
	if !deadline.IsZero() {
		if d, ok := ctx.Deadline(); !ok || deadline.Before(d) {
			var subCtx context.Context
			subCtx, cancel1 = context.WithDeadline(ctx, deadline)
			ctx = subCtx
		}
	}
	if oldCancel := d.Cancel; oldCancel != nil {
		var subCtx context.Context
		subCtx, cancel2 = context.WithCancel(ctx)
		go func() {
			select {
			case <-oldCancel:
				cancel2()
			case <-subCtx.Done():
			}
		}()
		ctx = subCtx
	}
	return ctx, func() {
		if cancel1 != nil {
			cancel1()
		}
		if cancel2 != nil {
			cancel2()
		}
	}
}

func (d *Dialer) dialTCPFromSocket(ctx context.Context, network string, laddr, raddr netip.AddrPort, b []byte) (*net.TCPConn, error) {
	ctx, cancel := d.dialCtx(ctx)
	defer cancel()

	c, err := d.dialSingle(ctx, network, laddr, raddr, b)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: opAddrPort(laddr), Addr: opAddrPort(raddr), Err: err}
	}
	return c, nil
}

func (d *Dialer) dialFromSocket(ctx context.Context, network, address string, b []byte) (net.Conn, error) {
	ctx, cancel := d.dialCtx(ctx)
	defer cancel()

	var laddr netip.AddrPort
	if d.LocalAddr != nil {
		la, ok := d.LocalAddr.(*net.TCPAddr)
		if !ok {
			return nil, &net.OpError{
				Op:     "dial",
				Net:    network,
				Source: nil,
				Addr:   nil,
				Err: &net.AddrError{
					Err:  "mismatched local address type",
					Addr: d.LocalAddr.String(),
				},
			}
		}
		laddr = la.AddrPort()
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}
	portNum, err := d.Resolver.LookupPort(ctx, network, port)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}
	ips, err := d.Resolver.LookupNetIP(ctx, ipNetwork(network), host)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}

	if laddr.IsValid() {
		for i, ip := range ips {
			if (ip.Is4() || ip.Is4In6()) != (laddr.Addr().Is4() || laddr.Addr().Is4In6()) {
				ips = slices.Delete(ips, i, i+1)
			}
		}
	}

	var primaries, fallbacks []netip.AddrPort
	if d.FallbackDelay >= 0 && network == "tcp" {
		var primaryLabel bool
		for i, ip := range ips {
			label := ip.Is4() || ip.Is4In6()
			addr := netip.AddrPortFrom(ip, uint16(portNum))
			if i == 0 || label == primaryLabel {
				primaryLabel = label
				primaries = append(primaries, addr)
			} else {
				fallbacks = append(fallbacks, addr)
			}
		}
	} else {
		primaries = make([]netip.AddrPort, len(ips))
		for i, ip := range ips {
			primaries[i] = netip.AddrPortFrom(ip, uint16(portNum))
		}
	}

	return d.dialParallel(ctx, network, laddr, primaries, fallbacks, b)
}

// dialParallel races two copies of dialSerial, giving the first a
// head start. It returns the first established connection and
// closes the others. Otherwise it returns an error from the first
// primary address.
func (d *Dialer) dialParallel(ctx context.Context, network string, laddr netip.AddrPort, primaries, fallbacks []netip.AddrPort, b []byte) (net.Conn, error) {
	if len(fallbacks) == 0 {
		return d.dialSerial(ctx, network, laddr, primaries, b)
	}

	returned := make(chan struct{})
	defer close(returned)

	type dialResult struct {
		net.Conn
		error
		primary bool
		done    bool
	}
	results := make(chan dialResult) // unbuffered

	startRacer := func(ctx context.Context, primary bool) {
		ras := primaries
		if !primary {
			ras = fallbacks
		}
		c, err := d.dialSerial(ctx, network, laddr, ras, b)
		select {
		case results <- dialResult{Conn: c, error: err, primary: primary, done: true}:
		case <-returned:
			if c != nil {
				c.Close()
			}
		}
	}

	var primary, fallback dialResult

	// Start the main racer.
	primaryCtx, primaryCancel := context.WithCancel(ctx)
	defer primaryCancel()
	go startRacer(primaryCtx, true)

	// Start the timer for the fallback racer.
	fallbackDelay := d.FallbackDelay
	if fallbackDelay == 0 {
		const defaultFallbackDelay = 300 * time.Millisecond
		fallbackDelay = defaultFallbackDelay
	}
	fallbackTimer := time.NewTimer(fallbackDelay)
	defer fallbackTimer.Stop()

	for {
		select {
		case <-fallbackTimer.C:
			fallbackCtx, fallbackCancel := context.WithCancel(ctx)
			defer fallbackCancel()
			go startRacer(fallbackCtx, false)

		case res := <-results:
			if res.error == nil {
				return res.Conn, nil
			}
			if res.primary {
				primary = res
			} else {
				fallback = res
			}
			if primary.done && fallback.done {
				return nil, primary.error
			}
			if res.primary && fallbackTimer.Stop() {
				// If we were able to stop the timer, that means it
				// was running (hadn't yet started the fallback), but
				// we just got an error on the primary path, so start
				// the fallback immediately (in 0 nanoseconds).
				fallbackTimer.Reset(0)
			}
		}
	}
}

// dialSerial connects to a list of addresses in sequence, returning
// either the first successful connection, or the first error.
func (d *Dialer) dialSerial(ctx context.Context, network string, laddr netip.AddrPort, ras []netip.AddrPort, b []byte) (net.Conn, error) {
	var firstErr error // The error from the first address is most relevant.

	for i, ra := range ras {
		select {
		case <-ctx.Done():
			return nil, &net.OpError{Op: "dial", Net: network, Source: opAddrPort(laddr), Addr: opAddrPort(ra), Err: ctx.Err()}
		default:
		}

		dialCtx := ctx
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			partialDeadline, err := partialDeadline(time.Now(), deadline, len(ras)-i)
			if err != nil {
				// Ran out of time.
				if firstErr == nil {
					firstErr = &net.OpError{Op: "dial", Net: network, Source: opAddrPort(laddr), Addr: opAddrPort(ra), Err: err}
				}
				break
			}
			if partialDeadline.Before(deadline) {
				var cancel context.CancelFunc
				dialCtx, cancel = context.WithDeadline(ctx, partialDeadline)
				defer cancel()
			}
		}

		c, err := d.dialSingle(dialCtx, network, laddr, ra, b)
		if err == nil {
			return c, nil
		}
		if firstErr == nil {
			// err is *net.OpError when TFO fallback happens.
			var ok bool
			firstErr, ok = err.(*net.OpError)
			if !ok {
				firstErr = &net.OpError{Op: "dial", Net: network, Source: opAddrPort(laddr), Addr: opAddrPort(ra), Err: err}
			}
		}
	}

	if firstErr == nil {
		firstErr = &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: errMissingAddress}
	}
	return nil, firstErr
}

func ipNetwork(network string) string {
	switch network {
	case "tcp4":
		return "ip4"
	case "tcp6":
		return "ip6"
	default:
		return "ip"
	}
}

func minNonzeroTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() || a.Before(b) {
		return a
	}
	return b
}

// deadline returns the earliest of:
//   - now+Timeout
//   - d.Deadline
//   - the context's deadline
//
// Or zero, if none of Timeout, Deadline, or context's deadline is set.
func (d *Dialer) deadline(ctx context.Context, now time.Time) (earliest time.Time) {
	if d.Timeout != 0 { // including negative, for historical reasons
		earliest = now.Add(d.Timeout)
	}
	if d, ok := ctx.Deadline(); ok {
		earliest = minNonzeroTime(earliest, d)
	}
	return minNonzeroTime(earliest, d.Deadline)
}

// partialDeadline returns the deadline to use for a single address,
// when multiple addresses are pending.
func partialDeadline(now, deadline time.Time, addrsRemaining int) (time.Time, error) {
	if deadline.IsZero() {
		return deadline, nil
	}
	timeRemaining := deadline.Sub(now)
	if timeRemaining <= 0 {
		return time.Time{}, os.ErrDeadlineExceeded
	}
	// Tentatively allocate equal time to each remaining address.
	timeout := timeRemaining / time.Duration(addrsRemaining)
	// If the time per address is too short, steal from the end of the list.
	const saneMinimum = 2 * time.Second
	if timeout < saneMinimum {
		if timeRemaining < saneMinimum {
			timeout = timeRemaining
		} else {
			timeout = saneMinimum
		}
	}
	return now.Add(timeout), nil
}
