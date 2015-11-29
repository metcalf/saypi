package respond

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"sync"
	"syscall"

	"goji.io"
	"golang.org/x/net/context"

	"github.com/juju/errors"
	"github.com/metcalf/saypi/log"
	"github.com/metcalf/saypi/metrics"
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
		metrics.Increment("respond_broken_pipe")
	} else if err != nil {
		panic(err)
	}
}

// Error returns a JSON response for the provided UserError and HTTP
// status code.
func Error(w http.ResponseWriter, status int, uerr usererrors.UserError) {
	content, err := usererrors.MarshalJSON(uerr)
	if err != nil {
		panic(err)
	}

	msg := json.RawMessage(content)
	Data(w, status, &msg)
}

// NotFound returns a JSON NotFound response with a 404 status.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	Error(w, http.StatusNotFound, usererrors.NotFound{})
}

var logMutex sync.Mutex

// WrapPanicC wraps a goji.Handler to catch panics, log relevant
// information and return an InternalFailure to the user.
func WrapPanicC(h goji.Handler) goji.Handler {
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err == nil {
				return
			}

			metrics.Increment("http.panics")

			id := fmt.Sprintf("%016x", rand.Int63())
			var lines []string

			pc, file, line, ok := runtime.Caller(3)
			if ok {
				f := runtime.FuncForPC(pc)
				lines = append(lines, fmt.Sprintf("%s:%d %s()", file, line, f.Name()))
			}

			if wrapped, ok := err.(*errors.Err); ok {
				for _, line := range wrapped.StackTrace() {
					lines = append(lines, line)
				}
			}

			data := map[string]interface{}{
				"PanicID": id,
			}

			if len(lines) > 1 {
				logMutex.Lock()
				defer logMutex.Unlock()
			}

			for _, line := range lines {
				log.Print("handled_panic", line, data)
			}

			Error(w, http.StatusInternalServerError, usererrors.InternalFailure{id})
		}()
		h.ServeHTTPC(ctx, w, r)
	})
}
