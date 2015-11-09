package auth_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"

	"github.com/metcalf/saypi/app"
	"github.com/metcalf/saypi/auth"
	"github.com/metcalf/saypi/mux"
)

var secret = []byte("shhh")

const (
	validUser = "DUtRr7IlHC-fd2wH8tfX_iLM8p8-3yeF4MTbc89B1lt41mk17sOlb6sg3JF_z6Sv"
	// This should be the same length as the validUser so it parses the same way
	invalidUser = "ABCDr7IlHC-fd2wH8tfX_iLM8p8-3yeF4MTbc89B1lt41mk17sOlb6sg3JF_z6Sv"
)

func TestFunctionalCreateAndGet(t *testing.T) {
	cfg := &app.Configuration{
		UserSecret: secret,
	}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}

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
		res.ID:      http.StatusNoContent,
		invalidUser: http.StatusNotFound,
		"notauser":  http.StatusNotFound,
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
	ctrl := auth.New(secret)

	var ctx, lastCtx context.Context

	handler := ctrl.WrapC(mux.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		lastCtx = ctx
		w.WriteHeader(http.StatusOK)
	}))

	lastCtx = nil
	ctx = context.TODO()

	testCases := map[string]int{
		"":        http.StatusUnauthorized,
		"invalid": http.StatusUnauthorized,
		validUser: http.StatusOK,
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
