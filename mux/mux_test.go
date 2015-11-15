package mux

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"golang.org/x/net/context"
)

func TestPatternMatch(t *testing.T) {
	// Adapted from https://github.com/bmizerany/pat/blob/master/mux_test.go
	// It isn't clear to me that some of these cases are actually desirable to support.
	cases := []struct {
		pattern, path string
		vars          url.Values
	}{
		{"/", "/", url.Values{}},
		{"/", "/nope", nil},
		{"/foo/:name", "/foo/bar", url.Values{"name": {"bar"}}},
		{"/foo/:name/baz", "/foo/bar", nil},
		{"/foo/:name/bar/", "/foo/keith/bar/baz", url.Values{"name": {"keith"}}},
		{"/foo/:name/bar/", "/foo/keith/bar/", url.Values{"name": {"keith"}}},
		{"/foo/:name/bar/", "/foo/keith/bar", nil},
		{"/foo/:name/baz", "/foo/bar/baz", url.Values{"name": {"bar"}}},
		{"/foo/:name/baz/:id", "/foo/bar/baz", nil},
		{"/foo/:name/baz/:id", "/foo/bar/baz/123", url.Values{"name": {"bar"}, "id": {"123"}}},
		{"/foo/:name/baz/:name", "/foo/bar/baz/123", url.Values{"name": {"bar", "123"}}},
		{"/foo/:name.txt", "/foo/bar.txt", url.Values{"name": {"bar"}}},
		{"/foo/:name", "/foo/:bar", url.Values{"name": {":bar"}}},
		{"/foo/:a:b", "/foo/val1:val2", url.Values{"a": {"val1"}, "b": {":val2"}}},
		{"/foo/:a.", "/foo/.", url.Values{"a": {""}}},
		{"/foo/:a:b", "/foo/:bar", url.Values{"a": {""}, "b": {":bar"}}},
		{"/foo/:a:b:c", "/foo/:bar", url.Values{"a": {""}, "b": {""}, "c": {":bar"}}},
		{"/foo/::name", "/foo/val1:val2", url.Values{"": {"val1"}, "name": {":val2"}}},
		{"/foo/:name.txt", "/foo/bar/baz.txt", nil},
		{"/foo/x:name", "/foo/bar", nil},
		{"/foo/x:name", "/foo/xbar", url.Values{"name": {"bar"}}},
	}

	for i, c := range cases {
		req, err := http.NewRequest("GET", c.path, nil)
		if err != nil {
			t.Fatal(err)
		}

		ctx, ok := Pattern("GET", c.pattern).Match(context.TODO(), req)
		if have, want := ok, (c.vars != nil); have != want {
			t.Errorf("%d: Expected match to be %t but got %t", i, want, have)
			continue
		}
		if c.vars != nil && len(c.vars) > 0 {
			m := FromContext(ctx)
			if m == nil {
				t.Errorf("%d: Expected context to contain a match", i)
			} else if !m.Matched() {
				t.Errorf("%d: Expected context to contain a successful match", i)
			} else if vars := m.Vars(); !reflect.DeepEqual(vars, c.vars) {
				t.Errorf("%d: Expected URL vars %v but got %v", i, c.vars, vars)
			}
		}
	}

	beforeCtx, ctxMatch := MatchContext(context.Background())
	ctx := ContextWithMatch(beforeCtx, &match{vars: url.Values{"foo": {"bar"}}})
	req, err := http.NewRequest("GET", "/baz", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Test method matching
	_, ok := Pattern("POST", "/:foo").Match(ctx, req)
	if ok {
		t.Error("Did not expect pattern to match")
	}

	// Test appending to an existing URL variable and getting a reference
	// to the match before matching.
	ctx, ok = Pattern("GET", "/:foo").Match(ctx, req)
	if !ok {
		t.Fatal("Expected pattern to match")
	}

	if m := FromContext(ctx); m == nil {
		t.Error("Expected context to contain match")
	} else if have, want := m.Vars()["foo"], []string{"bar", "baz"}; !reflect.DeepEqual(have, want) {
		t.Errorf("Expected URL variable to be %v but got %v", want, have)
	}

	if !ctxMatch.Matched() {
		t.Error("Expected match ref to have been set to matched")
	}
	if have, want := ctxMatch.Pattern(), "/:foo"; have != want {
		t.Errorf("Expected match ref to have pattern %q but got %q", want, have)
	}
}

func TestMux(t *testing.T) {
	var lastCtx context.Context
	m := New()

	m.RouteC(Pattern("GET", "/1"), HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		lastCtx = ctx
		w.WriteHeader(1)
	}))
	m.RouteFuncC(Pattern("GET", "/2"), func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		lastCtx = ctx
		w.WriteHeader(2)
	})
	m.Route(Pattern("GET", "/3"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastCtx = nil
		w.WriteHeader(3)
	}))
	m.RouteFunc(Pattern("GET", "/4"), func(w http.ResponseWriter, r *http.Request) {
		lastCtx = nil
		w.WriteHeader(4)
	})

	// ServeHTTPC
	testCases := map[int]bool{
		1: true,
		2: true,
		3: false,
		4: false,
	}

	for code, hasContext := range testCases {
		lastCtx = context.TODO()
		ctx := context.Background()

		req, err := http.NewRequest("GET", fmt.Sprintf("/%d", code), nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		m.ServeHTTPC(ctx, rr, req)

		if rr.Code != code {
			t.Errorf("%d: Got code %d", code, rr.Code)
		} else if hasContext && lastCtx == ctx {
			t.Errorf("%d: Expected context %s but got %s", code, ctx, lastCtx)
		} else if !hasContext && lastCtx != nil {
			t.Errorf("%d: Expected no context but got %s", code, lastCtx)
		}
	}

	// ServeHTTP
	lastCtx = nil
	req, err := http.NewRequest("GET", "/1", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)

	if rr.Code != 1 {
		t.Errorf("Expected 1 but got code %d", rr.Code)
	} else if lastCtx == nil {
		t.Errorf("Expected ServeHTTP to pass a TODO context")
	}

	// NotFoundHandler
	req, err = http.NewRequest("GET", "/notfound", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	m.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected %d but got code %d", http.StatusNotFound, rr.Code)
	}
}
