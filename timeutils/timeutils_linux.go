//go:build linux

package timeutils

import (
	"time"

	"golang.org/x/sys/unix"
)

// SetSystemTime sets the system time on Linux using a syscall.
func SetSystemTime(t time.Time) error {
	utc := t.UTC()
	tv := unix.Timeval{
		Sec:  utc.Unix(),
		Usec: int64(utc.Nanosecond() / 1000),
	}
	return unix.Settimeofday(&tv)
}
