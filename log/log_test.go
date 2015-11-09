package log_test

import (
	"bytes"
	stdlog "log"
	"testing"

	"github.com/metcalf/saypi/log"
)

func TestPrint(t *testing.T) {
	testCases := []struct {
		event, msg string
		data       map[string]interface{}
		expect     string
	}{
		{"foo", "bar", map[string]interface{}{"name": "bob"}, "foo: bar (name=\"bob\")\n"},
		{"foo", "", nil, "foo: \n"},
		{"foo", "bar", nil, "foo: bar\n"},
		{"foo", "bar", map[string]interface{}{}, "foo: bar\n"},
		{"foo", "", map[string]interface{}{"name": "bob"}, "foo: (name=\"bob\")\n"},
	}

	for i, testCase := range testCases {
		var buf bytes.Buffer
		logger := log.Logger{stdlog.New(&buf, "", 0)}

		logger.Print(testCase.event, testCase.msg, testCase.data)

		actual := buf.String()
		if actual != testCase.expect {
			t.Errorf("%d: Expected to print %q but got %q", i, testCase.expect, actual)
		}
	}
}
