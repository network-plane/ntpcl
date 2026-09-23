package timeutils

import "fmt"

// ErrNetwork indicates a network or transport failure.
type ErrNetwork struct {
	Err error
}

func (e *ErrNetwork) Error() string {
	return fmt.Sprintf("network error: %v", e.Err)
}

func (e *ErrNetwork) Unwrap() error {
	return e.Err
}

// ErrInvalidTime indicates the remote time source returned unusable data.
type ErrInvalidTime struct {
	Err error
}

func (e *ErrInvalidTime) Error() string {
	return fmt.Sprintf("invalid time: %v", e.Err)
}

func (e *ErrInvalidTime) Unwrap() error {
	return e.Err
}

// ErrPermission indicates missing privileges to set the system clock.
type ErrPermission struct {
	Err error
}

func (e *ErrPermission) Error() string {
	return fmt.Sprintf("permission denied: %v", e.Err)
}

func (e *ErrPermission) Unwrap() error {
	return e.Err
}
