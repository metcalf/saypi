package auth_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"goji.io"

	"golang.org/x/net/context"

	"github.com/metcalf/saypi/app"
	"github.com/metcalf/saypi/apptest"
	"github.com/metcalf/saypi/auth"
	"github.com/metcalf/saypi/client"
)

func TestAppCreateAndGet(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	cli := client.NewForTest(a)

	user, err := cli.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == "" {
		t.Fatal("received an empty user ID")
	}

	testCases := map[string]bool{
		user.ID:                 true,
		apptest.TestInvalidUser: false,
		"notauser":              false,
	}

	for id, expect := range testCases {
		actual, err := cli.UserExists(id)
		if err != nil {
			t.Fatalf("%s: %s", id, err)
		}

		if actual != expect {
			t.Errorf("exists=%t, expected %t", actual, expect)
		}
	}
}

func TestWrapC(t *testing.T) {
	ctrl, err := auth.New(apptest.TestSecret)
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
		"":                    http.StatusUnauthorized,
		"invalid":             http.StatusUnauthorized,
		apptest.TestValidUser: http.StatusOK,
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
