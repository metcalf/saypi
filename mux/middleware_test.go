package mux_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/metcalf/saypi/mux"
	"golang.org/x/net/context"
)

func teapot(f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "COFFEE" {
			w.WriteHeader(418)
		} else {
			f.ServeHTTP(w, r)
		}
	})
}

func sayit(line string) func(mux.HandlerC) mux.HandlerC {
	return func(f mux.HandlerC) mux.HandlerC {
		return mux.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
			lines, ok := ctx.Value("lines").([]string)
			if !ok {
				lines = make([]string, 0, 1)
			}
			lines = append(lines, line)

			f.ServeHTTPC(context.WithValue(ctx, "lines", lines), w, r)
		})
	}
}

func TestPassContext(t *testing.T) {
	var lastCtx context.Context

	wrapped := mux.PassContext(teapot)
	handler := wrapped(mux.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		lastCtx = ctx
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.Background()
	rr := httptest.NewRecorder()
	handler.ServeHTTPC(ctx, rr, &http.Request{Method: "COFFEE"})

	if rr.Code != 418 {
		t.Errorf("Expected code 418 but got %d", rr.Code)
	}
	if lastCtx != nil {
		t.Errorf("Unexpected context %q after calling teapot", lastCtx)
	}

	lastCtx = nil
	rr = httptest.NewRecorder()
	handler.ServeHTTPC(ctx, rr, &http.Request{Method: "TEA"})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected code %d but got %d", http.StatusOK, rr.Code)
	}
	if lastCtx != ctx {
		t.Errorf("Expected context %s to be passed but got %s", ctx, lastCtx)
	}
}

func TestMiddleware(t *testing.T) {
	var lastCtx context.Context

	mw := mux.NewMiddleware()
	mw.Add(teapot)
	mw.AddC(sayit("hi"))
	mw.AddC(sayit("there"))

	handler := mw.WrapC(mux.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		lastCtx = ctx
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.Background()
	rr := httptest.NewRecorder()
	handler.ServeHTTPC(ctx, rr, &http.Request{Method: "COFFEE"})

	if rr.Code != 418 {
		t.Errorf("Expected code 418 but got %d", rr.Code)
	}
	if lastCtx != nil {
		t.Errorf("Unexpected context %q after calling teapot", lastCtx)
	}

	lastCtx = nil
	rr = httptest.NewRecorder()
	handler.ServeHTTPC(ctx, rr, &http.Request{Method: "TEA"})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected code %d but got %d", http.StatusOK, rr.Code)
	}
	if lastCtx == nil {
		t.Errorf("Expected context to be passed but got nil")
	} else {
		t.Log(lastCtx)
		lines := lastCtx.Value("lines").([]string)
		want := []string{"hi", "there"}
		if !reflect.DeepEqual(lines, want) {
			t.Errorf("Expected context to contain lines %s but got %s", want, lines)
		}
	}
}
