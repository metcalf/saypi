// Package usererrors marshals and unmarshals structured errors bound
// for the end user of a service. It provides an interface for
// registering custom error types in addition to a set of common basic
// errors.
//
// Custom errors must conform to the UserError interface and should
// call Register in the init function of the package in which they are
// defined. The Code returned by the error must be unique across the
// application and any packages it imports. The Message returned
// should be a complete and properly puntuated sentence that can be
// displayed directly to the user. It should be generated exclusively
// from the contents of the error type, not provided by the
// caller. The provides the client the option of either displaying the
// generated error message directly to the end user or generating a
// custom message.
package usererrors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// UserError represents an error that can be returned to the client.
// UserErrors should be instantiated at the package-level with
// constant error strings.
type UserError interface {
	Code() string
	Message() string
}

type userError struct {
	CodeF    string `json:"code"`
	MessageF string `json:"message"`
}

func (e userError) Code() string    { return e.CodeF }
func (e userError) Message() string { return e.MessageF }

var registered map[string]reflect.Type

func init() {
	registered = make(map[string]reflect.Type)

	Register(InvalidParams{})
	Register(InternalFailure{})
	Register(ActionNotAllowed{})
	Register(NotFound{})
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
	content.userError = userError{uerr.Code(), uerr.Message()}

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

// Message returns a joined representation of parameter messages.
// When possible, the underlying data should be used instead to
// separate errors by parameter.
func (e InvalidParams) Message() string {
	if len(e) == 0 {
		return "Parameters you provided are invalid."
	}

	pms := make([]string, len(e))

	for i, pm := range e {
		var plural string
		if len(pm.Params) > 1 {
			plural = "s"
		}

		msg := pm.Message
		if msg == "" {
			msg = "provided is invalid"
		}

		var buf bytes.Buffer
		for i, param := range pm.Params {
			buf.WriteString(fmt.Sprintf("`%s`", param))
			switch i {
			case len(pm.Params) - 1:
				buf.WriteString(" ")
			case len(pm.Params) - 2:
				buf.WriteString(" and ")
			default:
				buf.WriteString(", ")
			}
		}

		pms[i] = fmt.Sprintf("Parameter%s %s%s.", plural, buf.String(), pm.Message)
	}

	return strings.Join(pms, " ")
}

// InternalFailure represents a prviate internal error.
type InternalFailure struct{}

// Code returns "internal_failure"
func (e InternalFailure) Code() string { return "internal_failure" }

// Message returns a generic internal error message
func (e InternalFailure) Message() string {
	return "Internal error encountered."
}

// ActionNotAllowed describes an action that is not permitted.
type ActionNotAllowed struct {
	Action string `json:"action"`
}

// Code returns "action_not_allowed"
func (e ActionNotAllowed) Code() string { return "action_not_allowed" }

// Message returns a string describing the disallowed action
func (e ActionNotAllowed) Message() string {
	return fmt.Sprintf("You may not %s.", e.Action)
}

// NotFound indicates that the requested resource could not be found.
type NotFound struct{}

// Code returns "not_found"
func (e NotFound) Code() string { return "not_found" }

// Message returns a generic not found message.
func (e NotFound) Message() string {
	return "The requested resource could not be found."
}

// AuthInvalid indicates that the authorization you provided is
// invalid.
type AuthInvalid struct{}

// Code returns "auth_invalid"
func (e AuthInvalid) Code() string { return "auth_invalid" }

// Message returns a generic unauthorized message.
func (e AuthInvalid) Message() string {
	return "The authorization token you provided is invalid."
}
