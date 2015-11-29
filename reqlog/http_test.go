package reqlog_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/metcalf/saypi/reqlog"

	"goji.io"

	"golang.org/x/net/context"
)

func TestWrapC(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	reqlog.SetLogger(logger)

	var setOK bool

	bare := goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		setOK = reqlog.SetContext(ctx, "hey", "oh")
		w.Write([]byte("foo"))
	})
	wrapped := reqlog.WrapC(bare)

	req, err := http.NewRequest("FOO", "/bar", nil)
	if err != nil {
		t.Fatal(err)
	}

	wrapped.ServeHTTPC(context.Background(), httptest.NewRecorder(), req)
	logged := buf.String()
	t.Log(logged)
	if !strings.Contains(logged, `http_status=200`) {
		t.Errorf("Expected http_status in line %s", logged)
	}
	if !strings.Contains(logged, `hey="oh"`) {
		t.Errorf("Expected to say hey oh in line %s", logged)
	}
	if !setOK {
		t.Error("SetContext should have set successfully.")
	}

	setOK = true
	buf.Reset()

	bare.ServeHTTPC(context.Background(), httptest.NewRecorder(), req)
	if buf.Len() != 0 {
		t.Errorf("Nothing should have been logged but got %s", buf.String())
	}
	if setOK {
		t.Errorf("SetContext should not have set successfully.")
	}

}
