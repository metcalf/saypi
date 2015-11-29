package respond_test

import (
	"bytes"
	"fmt"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"testing"

	"goji.io"
	"goji.io/pat"

	"github.com/juju/errors"
	"github.com/metcalf/saypi/log"
	"github.com/metcalf/saypi/respond"
)

func returnErr() error {
	return errors.New("with context")
}

func TestWrapPanic(t *testing.T) {
	var buf bytes.Buffer
	log.SetLogger(stdlog.New(&buf, "", 0))

	mux := goji.NewMux()

	mux.HandleFunc(pat.New("/safe"), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(pat.New("/panic"), func(w http.ResponseWriter, r *http.Request) {
		panic("hi there!")
	})
	mux.HandleFunc(pat.New("/trace"), func(w http.ResponseWriter, r *http.Request) {
		panic(errors.Trace(returnErr()))
	})

	mux.UseC(respond.WrapPanicC)

	req, err := http.NewRequest("FOO", "/safe", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusNoContent); err != nil {
		t.Error(err)
	}
	t.Log(rr.Body)
	t.Log(buf.String())

	buf.Reset()
	rr = httptest.NewRecorder()
	req.URL.Path = "/panic"
	mux.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusInternalServerError); err != nil {
		t.Error(err)
	}
	t.Log(rr.Body)
	t.Log(buf.String())

	buf.Reset()
	rr = httptest.NewRecorder()
	req.URL.Path = "/trace"
	mux.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusInternalServerError); err != nil {
		t.Error(err)
	}
	t.Log(rr.Body)
	t.Log(buf.String())
}

func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) error {
	if want == rr.Code {
		return nil
	}
	return fmt.Errorf("Expected status %d but got %d with body %s", want, rr.Code, rr.Body)
}
