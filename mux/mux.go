package mux

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"
)

func GetURLVar(ctx context.Context, key string) (string, bool) {
	v, ok := ctx.Value(key).(string)
	return v, ok
}

type HandlerC interface {
	ServeHTTPC(context.Context, http.ResponseWriter, *http.Request)
}

type HandlerFuncC func(context.Context, http.ResponseWriter, *http.Request)

func (f HandlerFuncC) ServeHTTPC(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	f(ctx, w, r)
}

func (f HandlerFuncC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(context.TODO(), w, r)
}

type Mux struct {
	routes          []*route
	NotFoundHandler HandlerC
}

func New() *Mux {
	return &Mux{}
}

func (m *Mux) RouteC(method, pat string, handler HandlerC) {
	m.routes = append(m.routes, &route{method, pat, handler})
}

func (m *Mux) RouteFuncC(method, pat string, handler HandlerFuncC) {
	m.routes = append(m.routes, &route{method, pat, handler})
}

func (m *Mux) ServeHTTPC(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	for _, route := range m.routes {
		if !strings.EqualFold(r.Method, route.method) {
			continue
		}

		if params, ok := route.try(r.URL.Path); ok {
			if len(params) > 0 {
				ctx = context.WithValue(ctx, "URLVars", params)
			}
			route.ServeHTTPC(ctx, w, r)
			return
		}
	}

	if m.NotFoundHandler != nil {
		m.NotFoundHandler.ServeHTTPC(ctx, w, r)
	} else {
		http.NotFound(w, r)
	}
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.ServeHTTPC(context.TODO(), w, r)
}

type route struct {
	method, pat string

	HandlerC
}

// Adapted from https://github.com/bmizerany/pat/blob/master/mux.go
func (r *route) try(path string) (url.Values, bool) {
	p := make(url.Values)
	var i, j int
	for i < len(path) {
		switch {
		case j >= len(r.pat):
			if r.pat != "/" && len(r.pat) > 0 && r.pat[len(r.pat)-1] == '/' {
				return p, true
			}
			return nil, false
		case r.pat[j] == ':':
			var name, val string
			var nextc byte
			name, nextc, j = match(r.pat, isAlnum, j+1)
			val, _, i = match(path, matchPart(nextc), i)
			p.Add(":"+name, val)
		case path[i] == r.pat[j]:
			i++
			j++
		default:
			return nil, false
		}
	}
	if j != len(r.pat) {
		return nil, false
	}
	return p, true
}

func matchPart(b byte) func(byte) bool {
	return func(c byte) bool {
		return c != b && c != '/'
	}
}

func match(s string, f func(byte) bool, i int) (matched string, next byte, j int) {
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
