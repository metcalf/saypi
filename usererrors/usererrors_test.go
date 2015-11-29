package usererrors_test

import (
	"bytes"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/metcalf/saypi/respond"
	"github.com/metcalf/saypi/usererrors"
)

func TestDecodeJSON(t *testing.T) {
	testcases := []usererrors.UserError{
		usererrors.InvalidParams{{
			Params:  []string{"foo"},
			Message: "hi there!",
		}},
		usererrors.InternalFailure{"myid"},
		usererrors.ActionNotAllowed{"doit"},
		usererrors.NotFound,
		usererrors.AuthRequired,
		usererrors.AuthInvalid,
	}

	for i, testcase := range testcases {
		rr := httptest.NewRecorder()
		respond.Error(rr, 1, testcase)

		res, err := usererrors.DecodeJSON(rr.Body)
		if err != nil {
			t.Errorf("%d: %s", i, err)
		} else if !reflect.DeepEqual(res, testcase) {
			t.Errorf("%d: err=%#v, expected %#v", i, res, testcase)
		}
	}

	unknownJSON := bytes.NewBufferString(`{"code":"foo","error":"bar"}`)
	if res, err := usererrors.DecodeJSON(unknownJSON); err != nil {
		t.Error(err)
	} else if res.Code() != usererrors.ErrUnknown {
		t.Errorf("code=%#v, expected ErrUnknown", res.Code)
	} else if have := res.Error(); have != "bar" {
		t.Errorf("error=%q, want %q", have, "bar")
	}

	invalidJSON := bytes.NewBufferString(`{"code":"invalid_params","error":"bar"}`)

	if _, err := usererrors.DecodeJSON(invalidJSON); err == nil {
		t.Error("expected an error decoding invalid JSON")
	}
}
