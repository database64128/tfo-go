//go:build darwin || freebsd || linux || windows

package tfo

import "time"

// aLongTimeAgo is a non-zero time, far in the past, used for immediate deadlines.
var aLongTimeAgo = time.Unix(0, 0)
