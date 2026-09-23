//go:build darwin

package timeutils

import (
	"time"

	"golang.org/x/sys/unix"
)

// SetSystemTime sets the system time on macOS using a syscall.
func SetSystemTime(t time.Time) error {
	utc := t.UTC()
	tv := unix.Timeval{
		Sec:  utc.Unix(),
		Usec: int32(utc.Nanosecond() / 1000),
	}
	return unix.Settimeofday(&tv)
}
