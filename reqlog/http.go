package reqlog

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"goji.io"

	"github.com/zenazn/goji/web/mutil"
	"golang.org/x/net/context"
)

const (
	httpDateFormat = "2006-01-02 15:04:05.000000"
	idCtxKey       = "log.ID"
	extraCtxKey    = "log.Extra"
)

var logger *log.Logger

func init() {
	SetLogger(log.New(os.Stderr, "", log.LstdFlags))
}

// SetLogger sets the underlying output logger
func SetLogger(lgr *log.Logger) {
	logger = lgr
}

func contextPrefix(ctx context.Context) string {
	id, ok := ctx.Value(idCtxKey).(string)
	if !ok {
		return ""
	}
	return fmt.Sprintf("[%s] ", id)
}

// Print prefixes the request ID, if any, and calls Print on the
// underlying logger.
func Print(ctx context.Context, v ...interface{}) {
	pfx := contextPrefix(ctx)

	if pfx != "" {
		v = append(v, "")
		copy(v[1:], v[0:])
		v[0] = pfx
	}

	logger.Print(v...)
}

// Printf prefixes the request ID, if any, and calls Printf on the
// underlying logger.
func Printf(ctx context.Context, format string, v ...interface{}) {
	pfx := contextPrefix(ctx)
	logger.Printf(pfx+format, v...)
}

// WrapC wraps a goji.Handler to log to the provided logger after the
// request completes. It adds a request ID to the context for logging
// with other functions in this package.
func WrapC(h goji.Handler) goji.Handler {
	// this takes the request and response, and tees off a copy of both
	// (truncated to a configurable length), and stores them in the request context
	// for later logging
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate a new request ID
		if _, ok := ctx.Value(idCtxKey).(string); !ok {
			id := mintActionID()
			ctx = context.WithValue(ctx, idCtxKey, id)
		}

		extra, ok := ctx.Value(extraCtxKey).(map[string]string)
		if !ok {
			extra = make(map[string]string)
			ctx = context.WithValue(ctx, extraCtxKey, extra)
		}

		w2 := mutil.WrapWriter(w)
		h.ServeHTTPC(ctx, w2, r)

		end := time.Now()

		remoteAddr, _, _ := net.SplitHostPort(r.RemoteAddr)
		reqTime := float64(end.Sub(start).Nanoseconds()) / float64(time.Second)

		var extraBuf bytes.Buffer
		for k, v := range extra {
			extraBuf.WriteString(fmt.Sprintf(" %s=%q", k, v))
		}

		Printf(ctx, "event=http_response time=%s remote_address=%q http_path=%q http_method=%q http_status=%d bytes_written=%d http_user_agent=%q request_time=%.6f%s",
			start.In(time.UTC).Format(httpDateFormat), remoteAddr, r.URL.Path, r.Method, w2.Status(), w2.BytesWritten(), r.UserAgent(), reqTime, extraBuf.String(),
		)
	})
}

// SetContext adds the key-value pair to the log data stored in the
// provided context.  It overwrites existing values and returns false
// if no log data is available in the context.
func SetContext(ctx context.Context, key, value string) bool {
	extra, ok := ctx.Value(extraCtxKey).(map[string]string)
	if ok {
		extra[key] = value
	}

	return ok
}
