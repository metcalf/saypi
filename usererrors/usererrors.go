package usererrors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// UserError represents an error that can be returned to the client.
// UserErrors should be instantiated at the package-level with
// constant error strings.
type UserError interface {
	error
	Code() string
}

type userError struct {
	CodeF  string `json:"code"`
	ErrorF string `json:"error"`
}

func (e userError) Code() string  { return e.CodeF }
func (e userError) Error() string { return e.ErrorF }

var registered map[string]reflect.Type

func init() {
	registered = make(map[string]reflect.Type)

	Register(InvalidParams{})
	Register(InternalFailure{})
	Register(ActionNotAllowed{})
	Register(NotFound{})
	Register(BearerAuthRequired{})
	Register(AuthInvalid{})
}

// Register associates an error code string with a concrete type
// for unmarshalling.
func Register(uerr UserError) error {
	code := uerr.Code()
	tp := reflect.TypeOf(uerr)

	if existing, ok := registered[code]; ok {
		if existing == tp {
			// Already registered
			return nil
		}
		return fmt.Errorf("error code %q is already registered to %s", code, tp)
	}

	registered[code] = tp
	return nil
}

// UnmarshalJSON parses a JSON-encoded UserError.  If the code of the
// error has been registered, the registered type is returned.
func UnmarshalJSON(data []byte) (UserError, error) {
	var uerr struct {
		userError
		Data json.RawMessage `json:"data,omitempty"`
	}

	if err := json.Unmarshal(data, &uerr); err != nil {
		return nil, err
	}

	tp, ok := registered[uerr.Code()]
	if !ok {
		return uerr, nil
	}

	val := reflect.New(tp)

	if uerr.Data != nil {
		if err := json.Unmarshal(uerr.Data, val.Interface()); err != nil {
			return nil, fmt.Errorf("unmarshaling error data: %s", err)
		}
	}
	return val.Elem().Interface().(UserError), nil
}

// MarshalJSON encodes the UserError as JSON. If the provided value is
// an array, map, slice or struct with at least one field it is
// marshalled into the `data` field.
func MarshalJSON(uerr UserError) ([]byte, error) {
	var content struct {
		userError
		Data interface{} `json:"data,omitempty"`
	}
	content.userError = userError{uerr.Code(), uerr.Error()}

	switch tp := reflect.Indirect(reflect.ValueOf(uerr)); tp.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		content.Data = uerr
	case reflect.Struct:
		if tp.NumField() > 0 {
			content.Data = uerr
		}
	}

	outer, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	return outer, nil
}

// InvalidParamsEntry represents a single error for InvalidParams
type InvalidParamsEntry struct {
	Params  []string `json:"params"`
	Message string   `json:"message"`
}

// InvalidParams represents a list of parameter validation
// errors. Each element in the list contains an explanation of the
// error and a list of the parameters that failed.
type InvalidParams []InvalidParamsEntry

// Code returns "invalid_params"
func (e InvalidParams) Code() string { return "invalid_params" }

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

// InternalFailure represents a prviate internal error.
type InternalFailure struct{}

// Code returns "internal_failure"
func (e InternalFailure) Code() string { return "internal_failure" }

// Error returns a generic internal error message
func (e InternalFailure) Error() string {
	return http.StatusText(http.StatusInternalServerError)
}

// ActionNotAllowed describes an action that is not permitted.
type ActionNotAllowed struct {
	Action string `json:"action"`
}

// Code returns "action_not_allowed"
func (e ActionNotAllowed) Code() string { return "action_not_allowed" }

// Error returns a string describing the disallowed action
func (e ActionNotAllowed) Error() string {
	return fmt.Sprintf("you may not %s", e.Action)
}

// NotFound indicates that the requested resource could not be found.
type NotFound struct{}

// Code returns "not_found"
func (e NotFound) Code() string { return "not_found" }

func (e NotFound) Error() string {
	return "the requested resource could not be found"
}

// BearerAuthRequired indicates that you must provide a Bearer token
// in the Authorization header.
type BearerAuthRequired struct{}

// Code returns "auth_required"
func (e BearerAuthRequired) Code() string { return "bearer_auth_required" }

func (e BearerAuthRequired) Error() string {
	return "you must provide a Bearer token in the Authorization header"
}

// AuthInvalid indicates that the authorization you provided is
// invalid.
type AuthInvalid struct{}

// Code returns "auth_invalid"
func (e AuthInvalid) Code() string { return "auth_invalid" }

func (e AuthInvalid) Error() string {
	return "the authorization token you provided is invalid"
}

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

/*
Examples from other apps:
type CardDeclined struct {
	Reason      string
	DeclineCode int
}

func (e CardDeclined) Code() errCode { return ErrCardDeclined }
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
