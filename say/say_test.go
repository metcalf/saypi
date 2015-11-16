package say_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
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
	defer a.Close()

	req, err := http.NewRequest("GET", "/animals", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusUnauthorized); err != nil {
		t.Error(err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth.TestValidUser))

	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Error(err)
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

func TestAppBuiltinMoods(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	req := newRequest(t, "GET", "/moods", nil)
	rr := httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	var listRes struct {
		Moods []say.Mood `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listRes); err != nil {
		t.Fatal(err)
	}
	if have, want := len(listRes.Moods), 8; have != want {
		t.Errorf("Expected %d built in moods but got %d", want, have)
	}

	req = newRequest(t, "GET", "/moods/borg", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

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

	// TODO: Attempting to update a builtin should return a sensible error
	if err := assertStatus(t, rr, http.StatusInternalServerError); err != nil {
		t.Error(err)
	}

	req = newRequest(t, "DELETE", "/moods/borg", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	// TODO: Attempting to delete a builtin should return a sensible error
	if err := assertStatus(t, rr, http.StatusInternalServerError); err != nil {
		t.Error(err)
	}
}

func TestAppMoods(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	// Get a non-existent mood
	req := newRequest(t, "GET", "/moods/cross", nil)
	rr := httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusNotFound); err != nil {
		t.Error(err)
	}

	// Create a mood
	expect := say.Mood{Name: "cross", Eyes: "><", Tongue: "<>", UserDefined: true}

	req = newRequest(t, "PUT", "/moods/cross", url.Values{
		"eyes":   {expect.Eyes},
		"tongue": {expect.Tongue},
	})
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	var got say.Mood
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expect, got) {
		t.Errorf("Created mood %#v not equal to expected %#v", got, expect)
	}

	// Get created mood
	req = newRequest(t, "GET", "/moods/cross", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expect, got) {
		t.Errorf("Retrieved mood %#v not equal to expected %#v", got, expect)
	}

	// List including created mood
	req = newRequest(t, "GET", "/moods", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	var listRes struct {
		Moods []say.Mood `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listRes); err != nil {
		t.Fatal(err)
	}
	if have, want := len(listRes.Moods), 9; have != want {
		t.Errorf("Expected %d moods but got %d", want, have)
	}

	// Update
	req = newRequest(t, "PUT", "/moods/cross", url.Values{
		"eyes": {"<>"},
	})
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Eyes != "<>" {
		t.Errorf("Eyes did not update %q", got.Eyes)
	}

	// Delete
	req = newRequest(t, "DELETE", "/moods/cross", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusNoContent); err != nil {
		t.Fatal(err)
	}

	req = newRequest(t, "GET", "/moods/cross", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusNotFound); err != nil {
		t.Error(err)
	}
}

func TestConversation(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	// CREATE
	heading := "top of the world"
	req := newRequest(t, "POST", "/conversations", url.Values{"heading": {heading}})
	rr := httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	var convo say.Conversation
	if err := json.Unmarshal(rr.Body.Bytes(), &convo); err != nil {
		t.Fatal(err)
	}
	if convo.Heading != heading {
		t.Errorf("Expected heading %q but got %q", heading, convo.Heading)
	}
	if len(convo.Lines) != 0 {
		t.Errorf("Unexpected lines in new conversation: %s", convo.Lines)
	}

	convoPath := fmt.Sprintf("/conversations/%s", convo.ID)

	// GET with no lines
	req = newRequest(t, "GET", convoPath, nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	var got say.Conversation
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(convo, got) {
		t.Errorf("Got %#v != created %#v", got, convo)
	}

	// Add line with builtin and created mood
	req = newRequest(t, "POST", convoPath+"/lines", url.Values{
		"animal": {"bunny"},
		"text":   {"hi there"},
	})
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	var line1 say.Line
	if err := json.Unmarshal(rr.Body.Bytes(), &line1); err != nil {
		t.Fatal(err)
	}
	t.Log(line1.Output)

	req = newRequest(t, "PUT", "/moods/cross", url.Values{
		"eyes":   {"><"},
		"tongue": {"<>"},
	})
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)
	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	req = newRequest(t, "POST", convoPath+"/lines", url.Values{
		"think": {"true"},
		"mood":  {"cross"},
		"text":  {"simmer down now"},
	})
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	var line2 say.Line
	if err := json.Unmarshal(rr.Body.Bytes(), &line2); err != nil {
		t.Fatal(err)
	}
	t.Log(line2.Output)

	// Get lines
	for i, line := range []say.Line{line1, line2} {
		path := fmt.Sprintf("%s/lines/%s", convoPath, line.ID)
		req = newRequest(t, "GET", path, nil)
		rr = httptest.NewRecorder()
		a.Srv.ServeHTTP(rr, req)

		if err := assertStatus(t, rr, http.StatusOK); err != nil {
			t.Error(err)
			continue
		}

		var got say.Line
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, line) {
			t.Errorf("%d: expected to get line %#v but got %#v", i, line, got)
		}
	}

	// Get with lines
	req = newRequest(t, "GET", convoPath, nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Lines) != 2 {
		t.Errorf("Expected 2 lines but got %d", len(got.Lines))
	} else {
		for i, line := range []say.Line{line1, line2} {
			if !reflect.DeepEqual(got.Lines[i], line) {
				t.Errorf("%d: expected line %#v but got %#v", i, line, got.Lines[i])
			}
		}
	}

	// List conversations
	req = newRequest(t, "GET", "/conversations", nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	var listRes struct {
		Convos []say.Conversation `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listRes); err != nil {
		t.Fatal(err)
	}
	if have, want := len(listRes.Convos), 1; have != want {
		t.Fatalf("Expected %d conversation but got %d", want, have)
	}

	got = listRes.Convos[0]
	if got.Heading != heading {
		t.Errorf("Expected heading %s but got %s", heading, got.Heading)
	}
	if len(got.Lines) > 0 {
		t.Errorf("Expected a list entry with no lines but got %d", len(got.Lines))
	}

	// Delete line
	path := fmt.Sprintf("%s/lines/%s", convoPath, line1.ID)
	req = newRequest(t, "DELETE", path, nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusNoContent); err != nil {
		t.Fatal(err)
	}

	req = newRequest(t, "GET", convoPath, nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Lines) != 1 {
		t.Errorf("Expected 1 line but got %d", len(got.Lines))
	}

	// delete conversation
	req = newRequest(t, "DELETE", convoPath, nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusNoContent); err != nil {
		t.Fatal(err)
	}

	req = newRequest(t, "GET", convoPath, nil)
	rr = httptest.NewRecorder()
	a.Srv.ServeHTTP(rr, req)

	if err := assertStatus(t, rr, http.StatusNotFound); err != nil {
		t.Fatal(err)
	}
}

func TestListing(t *testing.T) {
	// TODO: Test listing with before, after and limits
	// including a mix of builtin and user-created moods
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", auth.TestValidUser))

	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	return req
}

func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) error {
	if want == rr.Code {
		return nil
	}
	return fmt.Errorf("Expected status %d but got %d with body %s", want, rr.Code, rr.Body)
}
