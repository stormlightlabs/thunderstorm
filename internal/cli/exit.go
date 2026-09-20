package cli

import (
	"errors"
	"fmt"
)

// exitError carries the status a command leaves behind.
//
// The checks answer three questions through one number, and a caller that can
// only see "non-zero" cannot tell "your prose is bad" from "I could not read
// the file". Every other command keeps the single failure status it had.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }
func (e exitError) Unwrap() error { return e.err }

// findings is what a check returns when what it was given is wrong. The
// findings themselves are already printed; this is the summary line and the
// status.
func findings(format string, a ...any) error {
	return exitError{code: 1, err: fmt.Errorf(format, a...)}
}

// failed is what a check returns when it could not run at all.
func failed(format string, a ...any) error {
	return exitError{code: 2, err: fmt.Errorf(format, a...)}
}

// ExitCode is the process status for an error out of Execute.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit exitError
	if errors.As(err, &exit) {
		return exit.code
	}
	return 1
}
