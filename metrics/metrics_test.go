package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"golang.org/x/net/context"

	"github.com/metcalf/saypi/metrics"

	"goji.io"
	"goji.io/pat"
)

type stubBackend struct {
	data map[string]int
}

func (b *stubBackend) Increment(key string) {
	b.data[key]++
}

func (b *stubBackend) Reset() {
	b.data = make(map[string]int)
}

func TestMetrics(t *testing.T) {
	backend := &stubBackend{}
	metrics.SetBackend(backend)

	inner := goji.NewMux()
	inner.HandleFunc(pat.Get("/:baz"), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(1)
	})
	inner.UseC(metrics.WrapSubmuxC)

	outer := goji.NewMux()
	outer.HandleFunc(pat.Get("/foo"), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(2)
	})
	outer.HandleC(pat.New("/bar/*"), inner)
	outer.UseC(metrics.WrapC)

	cases := []struct {
		path   string
		expect map[string]int
	}{
		{"/foo", map[string]int{"foo.request": 1, "foo.response.2": 1}},
		{"/bar/baz", map[string]int{"bar.:baz.request": 1, "bar.:baz.response.1": 1}},
		{"/bar", map[string]int{}},
	}

	for _, testcase := range cases {
		backend.Reset()
		req, err := http.NewRequest("GET", testcase.path, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		outer.ServeHTTPC(context.Background(), rr, req)

		if have, want := backend.data, testcase.expect; !reflect.DeepEqual(have, want) {
			t.Errorf("%s: Expected %#v but got %#v", testcase.path, want, have)
		}
	}
}
