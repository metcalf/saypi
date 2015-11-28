package say

import (
	"reflect"
	"testing"

	"github.com/metcalf/saypi/dbutil"
)

const testUID = "u"

// For moods specifically,
// After/Before work when they are inside the range
// of built-ins and of user-defined. They both cross
// between ranges correctly.

func TestListMoods(t *testing.T) {
	tdb, db, err := dbutil.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer tdb.Close()
	defer db.Close()

	repo, err := newRepository(db)
	if err != nil {
		t.Fatal(err)
	}

	// Fixture some moods
	testMoods := []Mood{
		{"foo", " f", "oo", true, 0},
		{"bar", " b", "ar", true, 0},
		{"baz", " b", "az", true, 0},
	}

	moods := make([]Mood, len(testMoods)+len(builtinMoods))
	revMoods := make([]Mood, len(moods))
	for i, mood := range testMoods {
		err := repo.SetMood(testUID, &mood)
		if err != nil {
			t.Fatal(err)
		}
		moods[i] = mood
		revMoods[len(moods)-1-i] = mood
	}

	for i, mood := range builtinMoods {
		moods[i+len(testMoods)] = *mood
		revMoods[len(moods)-len(testMoods)-1-i] = *mood
	}

	moodNames := make([]string, len(moods))
	for i, mood := range revMoods {
		moodNames[i] = mood.Name
	}

	testcases := []struct {
		args    listArgs
		hasMore bool
		expect  []Mood
	}{
		// Works correctly without cursor
		0: {listArgs{Limit: 100}, false, moods},
		1: {listArgs{Limit: 4}, true, moods[0:4]},
		2: {listArgs{Limit: 0}, true, nil},
		// After returns one user-defined item correctly
		3: {listArgs{After: moods[0].Name, Limit: 1}, true, moods[1:2]},
		// After returns one built-in item correctly
		4: {listArgs{After: moods[2].Name, Limit: 1}, true, moods[3:4]},
		5: {listArgs{After: moods[3].Name, Limit: 1}, true, moods[4:5]},
		// After returns remaining items correctly ascending
		6: {listArgs{After: moods[0].Name, Limit: 20}, false, moods[1:]},
		// After the last returns nothing
		7: {listArgs{After: moods[len(moods)-1].Name, Limit: 1}, false, nil},
		// Before returns one user-defined item correctly
		8: {listArgs{Before: moods[2].Name, Limit: 1}, true, moods[1:2]},
		9: {listArgs{Before: moods[3].Name, Limit: 1}, true, moods[2:3]},
		// Before returns one built-in item correctly
		10: {listArgs{Before: moods[5].Name, Limit: 1}, true, moods[4:5]},
		// Before returns remaining items correctly descending
		11: {listArgs{Before: moods[len(moods)-1].Name, Limit: 20}, false, revMoods[1:]},
		// Before the first returns nothing
		12: {listArgs{Before: moods[0].Name, Limit: 1}, false, nil},
		// Zero limit returns nothing but indicates hasMore
		13: {listArgs{After: moods[1].Name, Limit: 0}, true, nil},
		14: {listArgs{Before: moods[1].Name, Limit: 0}, true, nil},
		15: {listArgs{After: moods[len(moods)-1].Name, Limit: 0}, false, nil},
		16: {listArgs{Before: moods[0].Name, Limit: 0}, false, nil},
	}

	for i, testcase := range testcases {
		actual, hasMore, err := repo.ListMoods(testUID, testcase.args)
		if err != nil {
			t.Errorf("%d: %s", i, err)
			continue
		}

		if hasMore != testcase.hasMore {
			t.Errorf("%d: hasMore=%t, expected %t", i, hasMore, testcase.hasMore)
		}

		// Coerce to nil because we don't care about the difference
		// between nil and empty slices.
		if len(actual) == 0 {
			actual = nil
		}

		if !reflect.DeepEqual(actual, testcase.expect) {
			t.Errorf("%d: expected list results\n\t%#v\nbut got\n\t%#v", i, testcase.expect, actual)
		}
	}

	// Check for the correct behavior with an invalid cursor
	for i, args := range []listArgs{{After: "nope"}, {Before: "nope"}} {
		_, _, err = repo.ListMoods(testUID, args)
		if err != errCursorNotFound {
			t.Errorf("%d: err=%s, expected errCursorNotFound", i, err)
		}
	}
}

func TestListConversations(t *testing.T) {
	tdb, db, err := dbutil.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer tdb.Close()
	defer db.Close()

	repo, err := newRepository(db)
	if err != nil {
		t.Fatal(err)
	}

	// Fixture some conversations
	headings := []string{"foo", "bar", "baz"}
	convos := make([]Conversation, len(headings))
	revConvos := make([]Conversation, len(headings))
	for i, heading := range headings {
		convo, err := repo.NewConversation(testUID, heading)
		if err != nil {
			t.Fatal(err)
		}
		convos[i] = *convo
		revConvos[len(headings)-1-i] = *convo
	}

	testcases := []struct {
		args    listArgs
		hasMore bool
		expect  []Conversation
	}{
		// Works correctly without cursor
		0: {listArgs{Limit: 5}, false, convos},
		1: {listArgs{Limit: 2}, true, convos[0:2]},
		2: {listArgs{Limit: 0}, true, nil},
		// After returns one item correctly
		3: {listArgs{After: convos[0].ID, Limit: 1}, true, convos[1:2]},
		// After returns all items correctly ascending
		4: {listArgs{After: convos[0].ID, Limit: 2}, false, convos[1:]},
		// After the last returns nothing
		5: {listArgs{After: convos[2].ID, Limit: 1}, false, nil},
		// Before returns one item correctly
		6: {listArgs{Before: convos[2].ID, Limit: 1}, true, revConvos[1:2]},
		// Before returns all items correctly descending
		7: {listArgs{Before: convos[2].ID, Limit: 2}, false, revConvos[1:]},
		// Before the first returns nothing
		8: {listArgs{Before: convos[0].ID, Limit: 1}, false, nil},
		// Zero limit returns nothing but indicates hasMore
		9:  {listArgs{After: convos[1].ID, Limit: 0}, true, nil},
		10: {listArgs{Before: convos[1].ID, Limit: 0}, true, nil},
		11: {listArgs{After: convos[2].ID, Limit: 0}, false, nil},
		12: {listArgs{Before: convos[0].ID, Limit: 0}, false, nil},
	}

	for i, testcase := range testcases {
		actual, hasMore, err := repo.ListConversations(testUID, testcase.args)
		if err != nil {
			t.Errorf("%d: %s", i, err)
			continue
		}

		if hasMore != testcase.hasMore {
			t.Errorf("%d: hasMore=%t, expected %t", i, hasMore, testcase.hasMore)
		}

		// Coerce to nil because we don't care about the difference
		// between nil and empty slices.
		if len(actual) == 0 {
			actual = nil
		}

		if !reflect.DeepEqual(actual, testcase.expect) {
			t.Errorf("%d: expected list results\n\t%#v\nbut got\n\t%#v", i, testcase.expect, actual)
		}
	}

	// Check for the correct behavior with an invalid cursor
	for i, args := range []listArgs{{After: "nope"}, {Before: "nope"}} {
		_, _, err = repo.ListConversations(testUID, args)
		if err != errCursorNotFound {
			t.Errorf("%d: err=%s, expected errCursorNotFound", i, err)
		}
	}
}
