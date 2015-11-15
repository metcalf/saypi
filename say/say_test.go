package say_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/metcalf/saypi/app"
	"github.com/metcalf/saypi/auth"
)

func TestAppGetAnimals(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", "/animals", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if have, want := rr.Code, http.StatusUnauthorized; have != want {
		t.Fatalf("Expected status %d but got %d with body %s", want, have, rr.Body)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.TestValidUser))

	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if have, want := rr.Code, http.StatusOK; have != want {
		t.Fatalf("Expected status %d but got %d with body %s", want, have, rr.Body)
	}

	var res struct {
		Animals []string `json:"animals"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}

	if have, want := len(res.Animals), 46; have != want {
		t.Fatalf("Only got %d of %d animals! %s", have, want, res.Animals)
	}
}
