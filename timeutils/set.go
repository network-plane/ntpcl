package timeutils

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// SetSystemTimeWrapper sets the system clock using syscalls or OS commands.
func SetSystemTimeWrapper(t time.Time, useSystemTools bool) error {
	if useSystemTools {
		return setSystemTimeWithCommand(t)
	}
	if err := SetSystemTime(t); err != nil {
		if isPermissionError(err) {
			return &ErrPermission{Err: fmt.Errorf("need elevated privileges to set system time (try sudo on Unix or SeSystemtimePrivilege on Windows): %w", err)}
		}
		return err
	}
	return nil
}

func setSystemTimeWithCommand(t time.Time) error {
	utc := t.UTC()

	switch runtime.GOOS {
	case "linux":
		formatted := utc.Format("2006-01-02 15:04:05")
		cmd := exec.Command("date", "-u", "-s", formatted)
		if err := cmd.Run(); err != nil {
			if isPermissionError(err) {
				return &ErrPermission{Err: fmt.Errorf("sudo may be required: %w", err)}
			}
			return err
		}
		return nil
	case "darwin":
		formatted := utc.Format("2006-01-02 15:04:05")
		cmd := exec.Command("date", "-u", "-f", "%Y-%m-%d %H:%M:%S", formatted)
		if err := cmd.Run(); err != nil {
			if isPermissionError(err) {
				return &ErrPermission{Err: fmt.Errorf("sudo may be required: %w", err)}
			}
			return err
		}
		return nil
	case "windows":
		iso := utc.Format("2006-01-02T15:04:05.000Z")
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf("Set-Date -Date '%s' -AsUTC", iso))
		if err := cmd.Run(); err != nil {
			if isPermissionError(err) {
				return &ErrPermission{Err: fmt.Errorf("SeSystemtimePrivilege may be required: %w", err)}
			}
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		return true
	}
	var pathErr *exec.ExitError
	if errors.As(err, &pathErr) {
		return pathErr.ExitCode() == 1
	}
	return false
}
