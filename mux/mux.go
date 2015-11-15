package mux

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"
)

const (
	matchKey = "mux.Match"
)

// ContextWithMatch merges the provided Match into the context,
// preserving references for MatchContext.
func ContextWithMatch(ctx context.Context, m Match) context.Context {
	val, ok := ctx.Value(matchKey).(*match)
	if !ok {
		val = &match{vars: make(url.Values)}
		ctx = context.WithValue(ctx, matchKey, val)
	}

	val.pattern = m.Pattern()
	val.matched = m.Matched()

	for k, vs := range m.Vars() {
		for _, v := range vs {
			val.vars.Add(k, v)
		}
	}

	return ctx
}

// MatchContext returns a Context and associated Match. If a match
// is already present in the context, it returns the same context and
// extracted match. If no match is present, a new Context and Match are
// returned. If a route matches later in the request, the Match will
// reflect that match.
func MatchContext(ctx context.Context) (context.Context, Match) {
	val, ok := ctx.Value(matchKey).(*match)
	if ok {
		return ctx, val
	}

	m := match{vars: make(url.Values)}
	return context.WithValue(ctx, matchKey, &m), &m
}

// FromContext returns the Match in the context, if any.
func FromContext(ctx context.Context) Match {
	val, ok := ctx.Value(matchKey).(*match)
	if !ok {
		return nil
	}
	return val
}

// HandlerC is an analog of http.Handler with a context parameter
type HandlerC interface {
	ServeHTTPC(context.Context, http.ResponseWriter, *http.Request)
}

type handlerWithC struct{ http.Handler }

func (h handlerWithC) ServeHTTPC(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	h.ServeHTTP(w, r)
}

type handlerWithoutC struct{ HandlerC }

func (h handlerWithoutC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTPC(context.TODO(), w, r)
}

// HandlerFuncC is an analog of http.HandlerFunc with a context parameter
// It implements both http.Handler and HandlerC
type HandlerFuncC func(context.Context, http.ResponseWriter, *http.Request)

// ServeHTTPC calls f(ctx, w, r)
func (f HandlerFuncC) ServeHTTPC(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	f(ctx, w, r)
}

// ServeHTTP calls f(context.TODO(), w, r)
func (f HandlerFuncC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(context.TODO(), w, r)
}

// Mux registers routes to be matched and dispatched
type Mux struct {
	routes []route
	// Configurable handler to be used when no route matches
	NotFoundHandler HandlerC
}

// New creates an empty Mux
func New() *Mux {
	return &Mux{
		NotFoundHandler: HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}),
	}
}

// RouteC adds a new route to a HandlerC
func (m *Mux) RouteC(matcher Matcher, handler HandlerC) {
	m.routes = append(m.routes, route{matcher, handler})
}

// RouteFuncC adds a new route to a HandlerFuncC
func (m *Mux) RouteFuncC(matcher Matcher, handler HandlerFuncC) {
	m.routes = append(m.routes, route{matcher, handler})
}

// Route adds a new route to an http.Handler, losing the request context.
func (m *Mux) Route(matcher Matcher, handler http.Handler) {
	m.routes = append(m.routes, route{matcher, handlerWithC{handler}})
}

// RouteFunc adds a new route to an http.HandlerFunc, losing the request context.
func (m *Mux) RouteFunc(matcher Matcher, handler http.HandlerFunc) {
	m.routes = append(m.routes, route{matcher, handlerWithC{handler}})
}

// ServeHTTPC dispatches the request to the handler in the matched
// route, preserving context. If no match is found, the
// NotFoundHandler is invoked.
func (m *Mux) ServeHTTPC(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	for _, route := range m.routes {
		if ctx, ok := route.Match(ctx, r); ok {
			route.ServeHTTPC(ctx, w, r)
			return
		}
	}

	m.NotFoundHandler.ServeHTTPC(ctx, w, r)
}

// ServeHTTP dispatches the request to the handler in the matched
// route with an empty TODO context.
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.ServeHTTPC(context.TODO(), w, r)
}

// A Matcher determines whether or not a given request matches some criteria.
type Matcher interface {
	// Returns new request context and true if the request satisfies
	// the pattern.  This function is free to examine both the request
	// and the context to make this decision.
	Match(context.Context, *http.Request) (context.Context, bool)
}

type match struct {
	pattern string
	vars    url.Values
	matched bool
}

func (m *match) Pattern() string  { return m.pattern }
func (m *match) Vars() url.Values { return m.vars }
func (m *match) Matched() bool    { return m.matched }

// Match is an interface to the route match for a request
type Match interface {
	// Pattern returns a string pattern that represents the matched route.
	Pattern() string
	// Vars returns any variables set by the matched route.
	Vars() url.Values
	// Matched returns true if a route match has been found.
	Matched() bool
}

type route struct {
	Matcher
	HandlerC
}

// Pattern creates a pat-style Matcher with support for URL variables.
// For example, PAT("GET", "/foo/:id/bar") would match a GET
// request to "/foo/myid/bar" and set the "id" url variable to "myid"
// in the context.
func Pattern(method, path string) Matcher {
	return &pathPattern{method, path}
}

type pathPattern struct {
	method, path string
}

// Adapted from https://github.com/bmizerany/pat/blob/master/mux.go
func (p *pathPattern) Match(ctx context.Context, r *http.Request) (context.Context, bool) {
	if !strings.EqualFold(p.method, r.Method) {
		return nil, false
	}

	path := r.URL.Path

	m := match{
		pattern: p.path,
		matched: true,
		vars:    make(url.Values),
	}

	var i, j int
	for i < len(path) {
		switch {
		case j >= len(p.path):
			if p.path != "/" && len(p.path) > 0 && p.path[len(p.path)-1] == '/' {
				return ContextWithMatch(ctx, &m), true
			}
			return nil, false
		case p.path[j] == ':':
			var name, val string
			var nextc byte
			name, nextc, j = matchPath(p.path, isAlnum, j+1)
			val, _, i = matchPath(path, matchPart(nextc), i)
			m.vars.Add(name, val)
		case path[i] == p.path[j]:
			i++
			j++
		default:
			return nil, false
		}
	}
	if j != len(p.path) {
		return nil, false
	}
	return ContextWithMatch(ctx, &m), true
}

func matchPart(b byte) func(byte) bool {
	return func(c byte) bool {
		return c != b && c != '/'
	}
}

func matchPath(s string, f func(byte) bool, i int) (matched string, next byte, j int) {
	j = i
	for j < len(s) && f(s[j]) {
		j++
	}
	if j < len(s) {
		next = s[j]
	}
	return s[i:j], next, j
}

func isAlpha(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func isAlnum(ch byte) bool {
	return isAlpha(ch) || isDigit(ch)
}
