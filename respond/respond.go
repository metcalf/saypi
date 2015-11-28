package respond

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"syscall"

	"github.com/metcalf/saypi/usererrors"
)

func isBrokenPipe(err error) bool {
	if err == nil {
		return false
	}
	if err == syscall.EPIPE {
		return true
	}
	if opErr, ok := err.(*net.OpError); ok && opErr.Err == syscall.EPIPE {
		return true
	}

	return false
}

// Data returns a JSON response with the provided data and HTTP status
// code.
func Data(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); isBrokenPipe(err) {
		log.Print("respond_broken_pipe", "unable to respond to client", nil)
	} else if err != nil {
		panic(err)
	}
}

// Error returns a JSON response for the provided UserError and HTTP
// status code. If the provided err implements ExtendedError, the
// value of Data is included as well.
func Error(w http.ResponseWriter, status int, err usererrors.UserError) {
	content := struct {
		Code  usererrors.ErrCode `json:"code"`
		Error string             `json:"error"`
		Data  interface{}        `json:"data,omitempty"`
	}{err.Code(), err.Error(), nil}

	datable, ok := err.(usererrors.ExtendedError)
	if ok {
		content.Data = datable.Data()
	}

	Data(w, status, content)
}

// NotFound returns a JSON NotFound response with a 404 status.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	Error(w, http.StatusNotFound, usererrors.NotFound)
}
