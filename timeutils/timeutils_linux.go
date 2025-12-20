//go:build linux
// +build linux

package timeutils

import (
	"syscall"
	"time"
)

// SetSystemTime sets the system time on Linux using syscalls.
func SetSystemTime(t time.Time) error {
	utc := t.UTC()
	tv := syscall.Timeval{
		Sec:  utc.Unix(),
		Usec: int64(utc.Nanosecond() / 1000),
	}
	return syscall.Settimeofday(&tv)
}
