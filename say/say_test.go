package say_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/metcalf/saypi/app"
	"github.com/metcalf/saypi/auth"
	"github.com/metcalf/saypi/say"
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

	assertStatus(t, rr, http.StatusUnauthorized)

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.TestValidUser))

	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	assertStatus(t, rr, http.StatusOK)

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

func TestAppBuiltinMoods(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}

	req := newRequest(t, "GET", "/moods", nil)
	rr := httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var listRes struct {
		Moods []say.Mood `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listRes); err != nil {
		t.Fatal(err)
	}
	if have, want := len(listRes.Moods), 7; have != want {
		t.Errorf("Expected %d built in moods but got %d", want, have)
	}

	req = newRequest(t, "GET", "/moods/borg", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	assertStatus(t, rr, http.StatusOK)

	var mood say.Mood
	if err := json.Unmarshal(rr.Body.Bytes(), &mood); err != nil {
		t.Fatal(err)
	}
	if mood.Eyes != "==" {
		t.Errorf("Borg eyes should be %q but got %q", "==", mood.Eyes)
	}
	if mood.UserDefined {
		t.Error("Built-in moods should set UserDefined")
	}

	req = newRequest(t, "PUT", "/moods/borg", url.Values{"eyes": {"--"}})
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	// TODO: Attempting to update a builtin should return an error
	// instead of failing silently.
	assertStatus(t, rr, http.StatusOK)

	req = newRequest(t, "DELETE", "/moods/borg", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	// TODO: Attempting to delete a builtin should return an error
	// instead of failing silently.
	assertStatus(t, rr, http.StatusNoContent)
}

func newRequest(t *testing.T, method, path string, form url.Values) *http.Request {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequest(method, path, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.TestValidUser))

	return req
}

func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	if want == rr.Code {
		return
	}
	t.Fatalf("Expected status %d but got %d with body %s", want, rr.Code, rr.Body)
}
