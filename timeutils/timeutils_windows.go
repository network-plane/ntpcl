//go:build windows

package timeutils

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows"
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

	if err := windows.SetSystemTime(&st); err != nil {
		return fmt.Errorf("SetSystemTime failed: %w", err)
	}
	return nil
}
