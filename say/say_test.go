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
	"github.com/metcalf/saypi/apptest"
	"github.com/metcalf/saypi/client"
	"github.com/metcalf/saypi/say"
	"github.com/metcalf/saypi/usererrors"
)

func TestAppGetAnimals(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	cli := client.NewForTest(a)

	_, err = cli.GetAnimals()
	if _, ok := err.(usererrors.BearerAuthRequired); !ok {
		t.Fatalf("request was not rejected due to missing auth: %s", err)
	}

	cli.SetAuthorization(apptest.TestValidUser)

	animals, err := cli.GetAnimals()
	if err != nil {
		t.Fatal(err)
	}

	if have, want := len(animals), 46; have != want {
		t.Fatalf("Only got %d of %d animals! %s", have, want, animals)
	}
}

func TestAppBuiltinMoods(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	cli := client.NewForTest(a)
	cli.SetAuthorization(apptest.TestValidUser)

	iter := cli.ListMoods(client.ListParams{})

	var names []string
	for iter.Next() {
		names = append(names, iter.Mood().Name)
	}
	if err := iter.Err(); err != nil {
		t.Fatal(err)
	}

	if have, want := len(names), 8; have != want {
		t.Errorf("Expected %d built in moods but got %d", want, have)
	}

	mood, err := cli.GetMood("borg")
	if err != nil {
		t.Fatal(err)
	}

	if mood.Eyes != "==" {
		t.Errorf("Borg eyes should be %q but got %q", "==", mood.Eyes)
	}
	if mood.UserDefined {
		t.Error("Built-in moods should set UserDefined")
	}

	err = cli.SetMood(&say.Mood{
		Name: "borg",
		Eyes: "--",
	})
	if _, ok := err.(usererrors.ActionNotAllowed); !ok {
		t.Errorf("expected an ActionNotAllowed but got %s", err)
	}

	err = cli.DeleteMood("borg")
	if _, ok := err.(usererrors.ActionNotAllowed); !ok {
		t.Errorf("expected an ActionNotAllowed but got %s", err)
	}
}

func TestAppMoods(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	cli := client.NewForTest(a)
	cli.SetAuthorization(apptest.TestValidUser)

	// Get a non-existent mood
	_, err = cli.GetMood("cross")
	if _, ok := err.(usererrors.NotFound); !ok {
		t.Errorf("expected NotFound for nonexistent mood but got %s", err)
	}

	// Create a mood
	expect := &say.Mood{Name: "cross", Eyes: "><", Tongue: "<>", UserDefined: true}

	got := &(*expect)
	if err := cli.SetMood(got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expect, got) {
		t.Errorf("created mood %#v not equal to expected %#v", got, expect)
	}

	// Get created mood
	got, err = cli.GetMood("cross")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expect, got) {
		t.Errorf("Retrieved mood %#v not equal to expected %#v", got, expect)
	}

	// List including created mood
	iter := cli.ListMoods(client.ListParams{})

	var names []string
	for iter.Next() {
		names = append(names, iter.Mood().Name)
	}
	if err := iter.Err(); err != nil {
		t.Fatal(err)
	}
	if have, want := len(names), 9; have != want {
		t.Errorf("Expected %d moods but got %d", want, have)
	}

	// Update
	got.Eyes = "<>"
	if err := cli.SetMood(got); err != nil {
		t.Fatal(err)
	}

	got, err = cli.GetMood(got.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Eyes != "<>" {
		t.Errorf("Eyes did not update %q", got.Eyes)
	}

	// Delete
	if err := cli.DeleteMood("cross"); err != nil {
		t.Fatal(err)
	}

	_, err = cli.GetMood("cross")
	if _, ok := err.(usererrors.NotFound); !ok {
		t.Errorf("expected NotFound after deleting mood but got %s", err)
	}
}

func TestConversation(t *testing.T) {
	cfg := &app.Configuration{}

	a, err := app.NewForTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	cli := client.NewForTest(a)
	cli.SetAuthorization(apptest.TestValidUser)

	// CREATE
	heading := "top of the world"
	req := newRequest(t, "POST", "/conversations", url.Values{"heading": {heading}})
	rr := httptest.NewRecorder()
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusOK); err != nil {
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
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusOK); err != nil {
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
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusOK); err != nil {
		t.Fatal(err)
	}

	var line1 say.Line
	if err := json.Unmarshal(rr.Body.Bytes(), &line1); err != nil {
		t.Fatal(err)
	}
	t.Log(line1.Output)

	mood := say.Mood{
		Name:   "cross",
		Eyes:   "><",
		Tongue: "<>",
	}
	if err := cli.SetMood(&mood); err != nil {
		t.Fatal(err)
	}

	req = newRequest(t, "POST", convoPath+"/lines", url.Values{
		"think": {"true"},
		"mood":  {"cross"},
		"text":  {"simmer down now"},
	})
	rr = httptest.NewRecorder()
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusOK); err != nil {
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
		a.ServeHTTP(rr, req)

		if err := apptest.AssertStatus(rr, http.StatusOK); err != nil {
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
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusOK); err != nil {
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
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusOK); err != nil {
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
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusNoContent); err != nil {
		t.Fatal(err)
	}

	req = newRequest(t, "GET", convoPath, nil)
	rr = httptest.NewRecorder()
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusOK); err != nil {
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
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusNoContent); err != nil {
		t.Fatal(err)
	}

	req = newRequest(t, "GET", convoPath, nil)
	rr = httptest.NewRecorder()
	a.ServeHTTP(rr, req)

	if err := apptest.AssertStatus(rr, http.StatusNotFound); err != nil {
		t.Fatal(err)
	}
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apptest.TestValidUser))

	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	return req
}
