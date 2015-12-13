package auth

import "github.com/metcalf/saypi/usererrors"

// BearerAuthRequired indicates that you must provide a Bearer token
// in the Authorization header.
type BearerAuthRequired struct{}

// Code returns "auth_required"
func (e BearerAuthRequired) Code() string { return "bearer_auth_required" }

// Message returns a human-readable description of the error.
func (e BearerAuthRequired) Message() string {
	return "You must provide a Bearer token in the Authorization header."
}

func init() {
	usererrors.Register(BearerAuthRequired{})
}
