package respond_test

import (
	"bytes"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goji.io"
	"goji.io/pat"

	"github.com/juju/errors"
	"github.com/metcalf/saypi/apptest"
	"github.com/metcalf/saypi/reqlog"
	"github.com/metcalf/saypi/respond"
	"github.com/metcalf/saypi/usererrors"
)

const testContext = "with super special context"

func returnErr() error {
	return errors.New(testContext)
}

func TestWrapPanic(t *testing.T) {
	var buf bytes.Buffer
	reqlog.SetLogger(stdlog.New(&buf, "", 0))

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

	if err := apptest.AssertStatus(rr, http.StatusNoContent); err != nil {
		t.Error(err)
	}

	buf.Reset()
	rr = httptest.NewRecorder()
	req.URL.Path = "/panic"
	mux.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusInternalServerError); err != nil {
		t.Error(err)
	}

	t.Log(rr.Body.String())
	uerr, err := usererrors.UnmarshalJSON(rr.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	_, ok := uerr.(usererrors.InternalFailure)
	if !ok {
		t.Errorf("expected an InternalFailure but got %#v", uerr)
	}

	t.Log(buf.String())

	buf.Reset()
	rr = httptest.NewRecorder()
	req.URL.Path = "/trace"
	mux.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusInternalServerError); err != nil {
		t.Error(err)
	}

	if !strings.Contains(buf.String(), testContext) {
		t.Errorf("error context %q not present in logs %s", testContext, buf.String())
	}

	t.Log(buf.String())
}
