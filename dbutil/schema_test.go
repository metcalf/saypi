package dbutil

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

var testSQL = []string{
	`CREATE TABLE bar (foo integer); INSERT INTO bar SELECT 1;`,
	`
CREATE TABLE bar (foo integer);
INSERT INTO bar SELECT 1
`,
}
var testStmts = []string{`CREATE TABLE bar (foo integer)`, `INSERT INTO bar SELECT 1`}

func TestReadSQL(t *testing.T) {
	file, err := ioutil.TempFile(os.TempDir(), "test_read_sql")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(file.Name())

	for _, str := range testSQL {
		if err := ioutil.WriteFile(file.Name(), []byte(str), os.ModePerm); err != nil {
			t.Fatal(err)
		}

		parsed, err := readSQL(file.Name())
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(testStmts, parsed) {
			t.Errorf("SQL was not parsed correctly. Expected:\n\t%#v\nGot:\n\t%#v",
				testStmts, parsed)
		}
	}
}

func TestSchemaApplies(t *testing.T) {
	tdb, db, err := NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer tdb.Close()
	defer db.Close()
}
