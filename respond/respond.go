package respond

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"syscall"

	"goji.io"
	"golang.org/x/net/context"

	"github.com/juju/errors"
	"github.com/metcalf/saypi/metrics"
	"github.com/metcalf/saypi/reqlog"
	"github.com/metcalf/saypi/usererrors"
)

var thisFile string

func init() {
	var ok bool
	_, thisFile, _, ok = runtime.Caller(1)
	if !ok {
		panic("Could not retrieve the name of the current file")
	}
}

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

func logError(ctx context.Context, err error, event string) {
	var lines []string

	for skip := 1; ; skip++ {
		pc, file, linenum, ok := runtime.Caller(skip)
		if ok && file != thisFile {
			f := runtime.FuncForPC(pc)
			line := fmt.Sprintf("%s:%d %s() event=%s", file, linenum, f.Name(), event)
			lines = append(lines, line)
			break
		}
	}

	if wrapped, ok := err.(*errors.Err); ok {
		for _, line := range wrapped.StackTrace() {
			lines = append(lines, line)
		}
	}

	if len(lines) > 1 {
		logMutex.Lock()
		defer logMutex.Unlock()
	}

	for _, line := range lines {
		reqlog.Print(ctx, line)
	}
}

// Data returns a JSON response with the provided data and HTTP status
// code.
func Data(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); isBrokenPipe(err) {
		reqlog.Print(ctx, "unable to respond to client. event=respond_broken_pipe")
		metrics.Increment("respond_broken_pipe")
	} else if err != nil {
		panic(err)
	}
}

// UserError returns a JSON response for the provided UserError and
// HTTP status code.
func UserError(ctx context.Context, w http.ResponseWriter, status int, uerr usererrors.UserError) {
	content, err := usererrors.MarshalJSON(uerr)
	if err != nil {
		panic(err)
	}

	reqlog.SetContext(ctx, "error_code", uerr.Code())

	msg := json.RawMessage(content)
	Data(ctx, w, status, &msg)
}

// NotFound returns a JSON NotFound response with a 404 status.
func NotFound(ctx context.Context, w http.ResponseWriter, _ *http.Request) {
	UserError(ctx, w, http.StatusNotFound, usererrors.NotFound{})
}

// InternalError returns an InternalFailure error with a 500 status code
// and logs the error stacktrace.
func InternalError(ctx context.Context, w http.ResponseWriter, err error) {
	uerr := usererrors.InternalFailure{}
	logError(ctx, err, uerr.Code())
	UserError(ctx, w, http.StatusInternalServerError, uerr)
}

var logMutex sync.Mutex

// WrapPanicC wraps a goji.Handler to catch panics, log relevant
// information and return an InternalFailure to the user.
func WrapPanicC(h goji.Handler) goji.Handler {
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			err, ok := recovered.(error)
			if !ok {
				err = fmt.Errorf("%s", err)
			}

			metrics.Increment("http.panics")
			logError(ctx, err, "panic")

			UserError(ctx, w, http.StatusInternalServerError, usererrors.InternalFailure{})
		}()
		h.ServeHTTPC(ctx, w, r)
	})
}
