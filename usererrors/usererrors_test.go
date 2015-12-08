package usererrors_test

import (
	"reflect"
	"testing"

	"github.com/metcalf/saypi/usererrors"
)

func TestDecodeJSON(t *testing.T) {
	testcases := []usererrors.UserError{
		0: usererrors.InvalidParams{{
			Params:  []string{"foo"},
			Message: "hi there!",
		}},
		1: usererrors.ActionNotAllowed{"doit"},
		2: usererrors.InternalFailure{},
		3: usererrors.NotFound{},
		4: usererrors.BearerAuthRequired{},
		5: usererrors.AuthInvalid{},
	}

	for i, testcase := range testcases {
		encoded, err := usererrors.MarshalJSON(testcase)

		t.Log(string(encoded))

		res, err := usererrors.UnmarshalJSON(encoded)
		if err != nil {
			t.Errorf("%d: %s", i, err)
		} else if !reflect.DeepEqual(res, testcase) {
			t.Errorf("%d: err=%#v, expected %#v", i, res, testcase)
		}
	}

	unknownJSON := []byte(`{"code":"foo","error":"bar"}`)
	if res, err := usererrors.UnmarshalJSON(unknownJSON); err != nil {
		t.Error(err)
	} else if res.Code() != "foo" {
		t.Errorf("code=%q, expected %q", res.Code(), "foo")
	} else if have := res.Error(); have != "bar" {
		t.Errorf("error=%q, want %q", have, "bar")
	}
}

type myErr struct {
	Some string `json:"some"`
}

func TestRegister(t *testing.T) {

}
