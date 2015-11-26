package auth_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"goji.io"

	"golang.org/x/net/context"

	"github.com/metcalf/saypi/app"
	"github.com/metcalf/saypi/auth"
)

func TestAppCreateAndGet(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	req, err := http.NewRequest("POST", "/users", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status %d but got %d with body %s", http.StatusOK, rr.Code, rr.Body)
	}

	var res struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}

	testCases := map[string]int{
		res.ID:               http.StatusNoContent,
		auth.TestInvalidUser: http.StatusNotFound,
		"notauser":           http.StatusNotFound,
	}

	for id, expect := range testCases {
		req, err = http.NewRequest("GET", fmt.Sprintf("/users/%s", id), nil)
		if err != nil {
			t.Fatal(err)
		}

		rr = httptest.NewRecorder()
		a.Srv.ServeHTTP(rr, req)

		if rr.Code != expect {
			t.Errorf("Expected retrieving user %q to return %d but got %d",
				id, expect, rr.Code)
		}
	}
}

func TestWrapC(t *testing.T) {
	ctrl, err := auth.New(auth.TestSecret)
	if err != nil {
		t.Fatal(err)
	}

	var ctx, lastCtx context.Context

	handler := ctrl.WrapC(goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		lastCtx = ctx
		w.WriteHeader(http.StatusOK)
	}))

	lastCtx = nil
	ctx = context.TODO()

	testCases := map[string]int{
		"":                 http.StatusUnauthorized,
		"invalid":          http.StatusUnauthorized,
		auth.TestValidUser: http.StatusOK,
	}

	for user, expect := range testCases {
		req, err := http.NewRequest("", "", nil)
		if err != nil {
			t.Fatal(err)
		}

		if user != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", user))
		}

		rr := httptest.NewRecorder()
		handler.ServeHTTPC(ctx, rr, req)

		if rr.Code != expect {
			t.Errorf("With authorization %q, expected status %d but got %d",
				user, expect, rr.Code)
		}
	}
}
