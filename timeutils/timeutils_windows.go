//go:build windows

package timeutils

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procSetSystemTime = modKernel32.NewProc("SetSystemTime")
)

// SetSystemTime sets the system time on Windows using the Windows API.
func SetSystemTime(t time.Time) error {
	utc := t.UTC()
	st := windows.Systemtime{
		Year:         uint16(utc.Year()),
		Month:        uint16(utc.Month()),
		Day:          uint16(utc.Day()),
		Hour:         uint16(utc.Hour()),
		Minute:       uint16(utc.Minute()),
		Second:       uint16(utc.Second()),
		Milliseconds: uint16(utc.Nanosecond() / 1e6),
	}

	r, _, err := procSetSystemTime.Call(uintptr(unsafe.Pointer(&st)))
	if r == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return fmt.Errorf("SetSystemTime failed: %w", err)
		}
		return fmt.Errorf("SetSystemTime failed")
	}
	return nil
}
