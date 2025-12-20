//go:build windows
// +build windows

package timeutils

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// SetSystemTime sets the system time on Windows using the Windows API.
func SetSystemTime(t time.Time) error {
	utc := t.UTC()
	systemTime := syscall.Systemtime{
		Year:         uint16(utc.Year()),
		Month:        uint16(utc.Month()),
		Day:          uint16(utc.Day()),
		Hour:         uint16(utc.Hour()),
		Minute:       uint16(utc.Minute()),
		Second:       uint16(utc.Second()),
		Milliseconds: uint16(utc.Nanosecond() / 1e6),
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setSystemTimeProc := kernel32.NewProc("SetSystemTime")

	r1, _, err := setSystemTimeProc.Call(uintptr(unsafe.Pointer(&systemTime)))
	if r1 == 0 {
		if err != nil {
			return err
		}
		// If SetSystemTime returns 0 (failure) but err is nil, return a generic error
		return fmt.Errorf("SetSystemTime failed")
	}
	return nil
}
