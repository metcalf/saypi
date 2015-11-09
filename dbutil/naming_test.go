package dbutil

import (
	"testing"
)

var cases = map[string]string{
	"Foo":          "foo",
	"FooBar":       "foo_bar",
	"fooBar":       "foo_bar",
	"FooBarID":     "foo_bar_id",
	"FooBarSID":    "foo_bar_sid",
	"FooBarSIDFoo": "foo_bar_sid_foo",

	"ID":  "id",
	"SID": "sid",
}

func TestMapperFunc(t *testing.T) {
	m := MapperFunc()

	for in, want := range cases {
		have := m(in)
		if have != want {
			t.Errorf("Expected %q to map to %q, but got %q", in, want, have)
		}
	}
}
