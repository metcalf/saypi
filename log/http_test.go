package log_test

import (
	"bytes"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goji.io"

	"github.com/metcalf/saypi/log"
	"golang.org/x/net/context"
)

func TestWrapC(t *testing.T) {
	var buf bytes.Buffer
	logger := log.Logger{stdlog.New(&buf, "", 0)}

	var setOK bool

	bare := goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		setOK = log.SetContext(ctx, "hey", "oh")
		w.WriteHeader(http.StatusOK)
	})
	wrapped := logger.WrapC(bare)

	req, err := http.NewRequest("FOO", "/bar", nil)
	if err != nil {
		t.Fatal(err)
	}

	wrapped.ServeHTTPC(context.Background(), httptest.NewRecorder(), req)
	logged := buf.String()
	if !strings.Contains(logged, `http_status="200"`) {
		t.Errorf("Expected to http_status in line %s", logged)
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
