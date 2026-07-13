package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies CLI failures for stable exit codes.
type Kind int

const (
	// KindGeneric maps to exit 1.
	KindGeneric Kind = iota
	// KindMisconfig maps to exit 2 (invalid flags, unknown profile, bad URLs).
	KindMisconfig
)

// Error is a classified application error.
type Error struct {
	Kind Kind
	Err  error
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Misconfig wraps err as a configuration/usage error.
func Misconfig(err error) error {
	if err == nil {
		return nil
	}
	return &Error{Kind: KindMisconfig, Err: err}
}

// Misconfigf formats a misconfiguration error.
func Misconfigf(format string, args ...any) error {
	return Misconfig(fmt.Errorf(format, args...))
}

// IsMisconfig reports whether err (or any wrapped error) is KindMisconfig.
func IsMisconfig(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind == KindMisconfig
	}
	return false
}
