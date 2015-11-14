package say

import (
	"fmt"
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	animals := listAnimals()

	if have, want := len(animals), 46; have != want {
		t.Errorf("Expected %d animals in list but got %d", want, have)
	}

	want := "bunny"
	for _, name := range animals {
		if name == want {
			return
		}
	}
	t.Errorf("Expected to find %q in list: %s", want, animals)
}

func TestSay(t *testing.T) {
	// Generate output with: cowsay foo | python -c "import sys; sys.stdout.write(repr(sys.stdin.read())[1:-1])" | pbcopy
	cases := []struct {
		cow, text, eyes, tongue string
		think                   bool
		expect                  string
	}{
		// Simple, single-line case
		{
			"",
			"foobarbaz",
			"",
			"",
			false,
			" ___________ \n< foobarbaz >\n ----------- \n        \\   ^__^\n         \\  (oo)\\_______\n            (__)\\       )\\/\\\n                ||----w |\n                ||     ||\n",
		},
		// Two-line with a different animal
		{
			"bunny",
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Donec faucibus.",
			"",
			"",
			false,
			" _________________________________________ \n/ Lorem ipsum dolor sit amet, consectetur \\\n\\ adipiscing elit. Donec faucibus.        /\n ----------------------------------------- \n  \\\n   \\   \\\n        \\ /\\\n        ( )\n      .( o ).\n",
		},
		// Three-line
		{
			"",
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Mauris non vulputate diam. Cras massa nunc.",
			"",
			"",
			false,
			" _________________________________________ \n/ Lorem ipsum dolor sit amet, consectetur \\\n| adipiscing elit. Mauris non vulputate   |\n\\ diam. Cras massa nunc.                  /\n ----------------------------------------- \n        \\   ^__^\n         \\  (oo)\\_______\n            (__)\\       )\\/\\\n                ||----w |\n                ||     ||\n",
		},
		// Customize eyes, tongue and think
		{
			"default",
			"This is my cow",
			"xo",
			"T ",
			true,
			" ________________ \n( This is my cow )\n ---------------- \n        o   ^__^\n         o  (xo)\\_______\n            (__)\\       )\\/\\\n             T  ||----w |\n                ||     ||\n",
		},
	}

	for i, testcase := range cases {
		cow, err := newCow(testcase.cow)
		if err != nil {
			t.Fatal(err)
		}

		said, err := cow.Say(testcase.text, testcase.eyes, testcase.tongue, testcase.think)
		if err != nil {
			t.Fatal(err)
		}

		if said != testcase.expect {
			diff := diffCows(testcase.expect, said)
			t.Errorf("%d: Expected\n\n%s\n\nbut got\n\n%s\n\n%s", i, testcase.expect, said, diff)
		}
	}
}

func diffCows(haveStr, wantStr string) string {
	haveLines := strings.Split(haveStr, "\n")
	wantLines := strings.Split(wantStr, "\n")

	var diff []string

	for i := range wantLines {
		have, want := haveLines[i], wantLines[i]
		if have == want {
			continue
		}
		diff = append(diff, fmt.Sprintf("%q", have), fmt.Sprintf("%q", want), "")
	}
	return strings.Join(diff, "\n")
}
