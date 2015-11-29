package metrics

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/codahale/metrics"
	"github.com/zenazn/goji/web/mutil"

	"goji.io/middleware"

	"goji.io"
	"golang.org/x/net/context"
)

const patCtxKey = "metrics.Pattern"

// Backend acts as a sink for metrics
type Backend interface {
	Increment(string)
}

type codaBackend struct{}

func (b codaBackend) Increment(key string) {
	metrics.Counter(key).Add()
}

var backend = Backend(codaBackend{})

// Increment calls the Increment method on the current package-level backend.
func Increment(key string) {
	backend.Increment(key)
}

// WrapC wraps a handler to track request counts and response status
// code counts namespaced by goji Pattern. It will only include
// patterns that implemnt fmt.Stringer. For example, if a request
// matches the pattern /foo/:bar and returns a 204 status code, it
// will increment "foo.:bar.request" and "foo.:bar.response.204".
//
// WrapC is only safe to use once per request. If you have nested
// muxes, use WrapC in the outer mux and WrapSubmuxC on the inner mux.
func WrapC(h goji.Handler) goji.Handler {
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		var patterns []goji.Pattern

		curr := middleware.Pattern(ctx)
		if curr != nil {
			patterns = append(patterns, curr)
		}

		ctx = context.WithValue(ctx, patCtxKey, &patterns)
		w2 := mutil.WrapWriter(w)
		h.ServeHTTPC(ctx, w2, r)

		patStrs := make([]string, len(patterns))
		for i, pattern := range patterns {
			patStr, ok := pattern.(fmt.Stringer)
			if !ok {
				continue
			}

			patStrs[i] = strings.TrimSuffix(patStr.String(), "/*")
		}

		fullPatStr := strings.Trim(strings.Replace(path.Join(patStrs...), "/", ".", -1), ".")

		if fullPatStr != "" {
			Increment(fmt.Sprintf("%s.request", fullPatStr))
			Increment(fmt.Sprintf("%s.response.%d", fullPatStr, w2.Status()))
		}
	})
}

// WrapSubmuxC is a helper for using WrapC with nested muxes. It
// stores the pattern matched in the current mux but does not track
// any metrics independently. It should be included in every mux
// except the outer one.
func WrapSubmuxC(h goji.Handler) goji.Handler {
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		patterns, ok := ctx.Value(patCtxKey).(*[]goji.Pattern)
		if ok {
			curr := middleware.Pattern(ctx)
			if curr != nil {
				*patterns = append(*patterns, curr)
			}
		}

		h.ServeHTTPC(ctx, w, r)
	})
}

// SetBackend configures the package-global metrics Backend
func SetBackend(b Backend) {
	backend = b
}
