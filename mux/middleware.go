package mux

import (
	"net/http"

	"golang.org/x/net/context"
)

// PassContext translates an http.Handler wrapper/middleware function
// into one that is context aware by passing the context around the
// old wrapper.
func PassContext(f func(http.Handler) http.Handler) func(HandlerC) HandlerC {
	return func(fc HandlerC) HandlerC {
		return HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
			f(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fc.ServeHTTPC(ctx, w, r)
			})).ServeHTTP(w, r)
		})
	}
}

// WithoutC returns an http.Handler that calls the passed HandlerC
// with a TODO context.
func WithoutC(h HandlerC) http.Handler {
	return handlerWithoutC{h}
}

// Middleware represents a stack of functions to wrap a HandlerC
type Middleware struct {
	stack []func(HandlerC) HandlerC
}

// NewMiddleware creates an empty Middleware
func NewMiddleware() *Middleware {
	return &Middleware{}
}

// WrapC wraps a HandlerC in each function added to the middleware
// stack in order.
func (mw *Middleware) WrapC(handler HandlerC) HandlerC {
	for i := len(mw.stack) - 1; i >= 0; i-- {
		handler = mw.stack[i](handler)
	}

	return handler
}

// Wrap wraps a HandlerC in each function added to the middleware
// stack and returns an http.Handler for use with e.g. a stdlib server.
func (mw *Middleware) Wrap(handler HandlerC) http.Handler {
	return WithoutC(mw.WrapC(handler))
}

// AddC adds a wrapper function to the end of the middleware.
func (mw *Middleware) AddC(f func(HandlerC) HandlerC) {
	mw.stack = append(mw.stack, f)
}

// Add is an alias for AddC that calls PassContext to handle wrappers
// that are not context-aware.
func (mw *Middleware) Add(f func(http.Handler) http.Handler) {
	mw.AddC(PassContext(f))
}
