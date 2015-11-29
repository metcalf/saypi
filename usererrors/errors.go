package usererrors

import (
	"fmt"
	"net/http"
	"strings"
)

type ErrCode string

const (
	ErrInvalidParams    ErrCode = "invalid_params"
	ErrNotFound         ErrCode = "not_found"
	ErrAuthRequired     ErrCode = "auth_required"
	ErrAuthInvalid      ErrCode = "auth_invalid"
	ErrInternalFailure  ErrCode = "internal_failure"
	ErrActionNotAllowed ErrCode = "action_not_allowed"
)

// UserError represents an error that can be returned to the client.
// UserErrors should be instantiated at the package-level with
// constant error strings. If you need to communicate dynamic information
// with the error string, use an ExtendedError to encode the dynamic
// information in a machine-readable form.
type UserError interface {
	error
	Code() ErrCode
}

// ExtendedError represents a UserError with extra, machine-readable
// data that can be returned to the client.
type ExtendedError interface {
	UserError
	Data() interface{}
}

type userError struct {
	code    ErrCode
	message string
}

func (e userError) Code() ErrCode { return e.code }
func (e userError) Error() string { return e.message }

// InvalidParams represents a list of parameter validation
// errors. Each element in the list contains an explanation of the
// error and a list of the parameters that failed.
type InvalidParams []struct {
	Message string   `json:"message"`
	Params  []string `json:"params"`
}

// Code returns ErrInvalidParams
func (e InvalidParams) Code() ErrCode { return ErrInvalidParams }

// Error returns a joined representation of parameter messages.
// When possible, the underlying data should be used instead to
// separate errors by parameter.
func (e InvalidParams) Error() string {
	pms := make([]string, len(e))

	for i, pm := range e {
		pms[i] = fmt.Sprintf("%s: %s.", strings.Join(pm.Params, ", "), pm.Message)
	}

	return strings.Join(pms, " ")
}

// Data returns the value itself.
func (e InvalidParams) Data() interface{} { return e }

// InternalFailure represents a prviate error with
// a unique identifier that can be referenced in private application logs.
type InternalFailure struct {
	ID string `json:"id"`
}

// Code returns ErrInternalFailure
func (e InternalFailure) Code() ErrCode { return ErrInternalFailure }

// Error returns a generic internal error message
func (e InternalFailure) Error() string { return http.StatusText(http.StatusInternalServerError) }

// Data returns the value itself.
func (e InternalFailure) Data() interface{} { return e }

// ActionNotAllowed represents an ErrActionNotAllowed containing
// a description of the action that is not permitted.
type ActionNotAllowed struct {
	Action string `json:"action"`
}

// Code returns ErrActionNotAllowed
func (e ActionNotAllowed) Code() ErrCode { return ErrActionNotAllowed }

// Error returns a string describing the disallowed action
func (e ActionNotAllowed) Error() string {
	return fmt.Sprintf("you may not %s", e.Action)
}

// Data returns the value itself.
func (e ActionNotAllowed) Data() interface{} { return e }

// NotFound is an error of code ErrNotFound indicating that
// the requested resource could not be found.
var NotFound = userError{ErrNotFound, "the requested resource could not be found"}

// AuthRequired is an error of code ErrAuthRequired indicating that
// you must provide a Bearer token in an Authorization header.
var AuthRequired = userError{ErrAuthRequired, "you must provide a Bearer token in an Authorization header"}

// AuthInvalid is an error of code ErrAuthInvalid indicating that
// the authorization you provided is invalid
var AuthInvalid = userError{ErrAuthInvalid, "the authorization token you provided is invalid"}

// Don't necessarily have a Cause,
// Never need a Cause if it's a UserError?
// Probably only want to wrap errors at lower levels and return user errors
// at a higher level. Those facilities should be totally separate.
// Don't care about propogating UserError around the stack

// Need to be able to turn specific client fields red for invalid params
// Return extra info like decline codes
// Low-level authorization code should not have to know whether a user
// is gated in to seeing detailed decline codes before propogating the
// result to an upper level of the stack.

// Should be able to type switch in Go code and string switch in
// parsing code.
// Would be great if all were stringers in a similar way
// Basically there are some things for which we want custom
// types and others for which we really don't care and just want code+message automatically
// Want Golang clients to be able to reify errors into original types
// Want non-Golang clients to be able to parse machine-readable error codes (even better would be to make code-gen easy, codes as data)
// Possible error types should be pretty obvious from package docs

/*
Examples from other apps:
type CardDeclined struct {
	Reason      string
	DeclineCode int
}

func (e CardDeclined) Code() ErrCode { return ErrCardDeclined }
func (e CardDeclined) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	return "Your card was declined."
}

// NoParseableImage returns an error of type ErrNoParseableImage indicating that
// no parseable image could be retrieved for the provided URL
var NoParseableImage = userError{ErrNoParseableImage, "no parseable image could be retrieved for the provided URL"}
*/
