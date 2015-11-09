package dbutil

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/codahale/testdb"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Load the Postgres drivers for convenience
)

// DriverName is the name of the database driver used by NewTestDB
const DriverName = "postgres"

// DefaultDataSource is the default database DSN prefix used by NewTestDB
var DefaultDataSource = testdb.Env("TEST_PG", "sslmode=disable")

// NewTestDB connects to the default DBMS, creates a new database using testdb,
// and loads the application's schema.
func NewTestDB() (*testdb.TestDB, *sqlx.DB, error) {
	stmts, err := readSQL("../schema.sql")
	if err != nil {
		return nil, nil, err
	}

	tdb, err := testdb.Open(DriverName, DefaultDataSource+" dbname=postgres")
	if err != nil {
		return nil, nil, err
	}

	db, err := sql.Open(DriverName, DefaultDataSource+" dbname="+tdb.Name())
	if err != nil {
		tdb.Close()
		return nil, nil, err
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			tdb.Close()
			return nil, nil, fmt.Errorf("%#v\nExecuting: %s", err, stmt)
		}
	}

	dbx := sqlx.NewDb(db, DriverName)

	return tdb, dbx, nil
}

// ReadSQL reads a file at the provided path and parses it into separate
// SQL statement strings. It does not currently handle semicolons within
// statements such as within a string literal.
func readSQL(filename string) ([]string, error) {
	var stmts []string

	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Unable to read %s: %v", filename, err)
	}

	scanner := bufio.NewScanner(f)
	scanner.Split(scanStmts)

	for scanner.Scan() {
		stmts = append(stmts, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Unable to parse input from %s: %v", filename, err)
	}

	return stmts, nil
}

func scanStmts(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading spaces.
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !unicode.IsSpace(r) {
			break
		}
	}
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	end := start
	// Scan until semicolon, marking end of statement.
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if r == ';' {
			return i + width, data[start:i], nil
		} else if !unicode.IsSpace(r) {
			end = i + 1
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated statement. Return it.
	if atEOF && len(data) > start {
		return len(data), data[start:end], nil
	}
	// Request more data.
	return 0, nil, nil
}
