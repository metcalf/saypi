package client

import (
	"fmt"

	"github.com/metcalf/saypi/usererrors"
)

type userError struct {
	usererrors.UserError
}

func (e userError) Error() string {
	return fmt.Sprintf("saypi client: received error %q", e.Message())
}

// UserError returns the underlying UserError returned by the
// client request if the error was generated from a UserError response.
// Otherwise, it returns nil.
func UserError(err error) usererrors.UserError {
	if uerr, ok := err.(userError); ok {
		return uerr.UserError
	}

	return nil
}
