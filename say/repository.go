package say

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/juju/errors"
	"github.com/lib/pq"
)

const (
	maxInsertRetries = 16
	convoIDPrefix    = "cv_"
	lineIDPrefix     = "ln_"
	dbErrDupUnique   = "23505"

	listMoods = `
SELECT id as int_id, name, eyes, tongue
FROM moods
WHERE user_id = :user_id AND
  (:cursor_id < 0 OR id %s :cursor_id)
ORDER BY 1 %s
LIMIT :limit + 1
`
	findMood = `
SELECT id as int_id, eyes, tongue, name
FROM moods
WHERE user_id = :user_id AND lower(name) = lower(:name)
`
	deleteMood = `
DELETE FROM moods
WHERE user_id = :user_id AND lower(name) = lower(:name)
`
	// TODO: Racy upsert
	setMood = `
WITH
updated as (
  UPDATE moods SET eyes = :eyes, tongue = :tongue
  WHERE user_id = :user_id AND lower(name) = lower(:name)
  RETURNING id
),
inserted as (
  INSERT INTO moods (user_id, name, eyes, tongue)
  SELECT :user_id, lower(:name), :eyes, :tongue
  WHERE NOT EXISTS (SELECT * FROM updated)
  RETURNING id
)
SELECT id FROM updated UNION ALL SELECT id FROM inserted
`

	listConvos = `
SELECT id as int_id, public_id as id, heading
FROM conversations
WHERE user_id = :user_id AND
  (:cursor_id < 0 OR id %s :cursor_id)
ORDER BY 1 %s
LIMIT :limit
`
	insertConvo = `
INSERT INTO conversations (public_id, user_id, heading)
SELECT :public_id, :user_id, :heading
RETURNING id
`
	getConvo = `
SELECT id as int_id, public_id as id, heading FROM conversations
WHERE user_id = :user_id AND public_id = :public_id
`
	deleteConvo = `
DELETE FROM conversations WHERE user_id = :user_id AND public_id = :public_id
`

	findConvoLines = `
SELECT public_id as id, animal, think, text, mood_name, eyes, tongue
FROM lines
LEFT JOIN moods ON lines.mood_id = moods.id
WHERE conversation_id = :id
ORDER BY lines.id ASC
`
	insertLine = `
INSERT INTO LINES (public_id, animal, think, text, mood_name, mood_id, conversation_id)
SELECT :public_id, :animal, :think, :text, :mood_name, :mood_id, :conversation_id
`
	getLine = `
SELECT lines.public_id as id, animal, think, text, mood_name, eyes, tongue
FROM lines
LEFT JOIN moods ON lines.mood_id = moods.id
INNER JOIN conversations ON lines.conversation_id = conversations.id
WHERE
  conversations.public_id = :convo_id AND
  conversations.user_id = :user_id AND
  lines.public_id = :line_id
`
	deleteLine = `
DELETE FROM lines
USING conversations
WHERE
  lines.conversation_id = conversations.id AND
  conversations.public_id = :convo_id AND
  conversations.user_id = :user_id AND
  lines.public_id = :line_id
`
)

var errCursorNotFound = errors.New("Invalid cursor")
var errBuiltinMood = errors.New("Cannot modify built-in moods")

type repository struct {
	db      *sqlx.DB
	closers []io.Closer

	listMoodsAsc, listMoodsDesc, findMood, deleteMood, setMood        *sqlx.NamedStmt
	listConvosAsc, listConvosDesc, insertConvo, getConvo, deleteConvo *sqlx.NamedStmt
	findConvoLines, insertLine, getLine, deleteLine                   *sqlx.NamedStmt
}

type listArgs struct {
	Before, After string
	Limit         int
}

var builtinMoods = []*Mood{
	{"default", "oo", "  ", false, 0},
	{"borg", "==", "  ", false, 0},
	{"dead", "xx", "U ", false, 0},
	{"greedy", "$$", "  ", false, 0},
	{"stoned", "**", "U ", false, 0},
	{"tired", "--", "  ", false, 0},
	{"wired", "OO", "  ", false, 0},
	{"young", "..", "  ", false, 0},
}

type moodRec struct {
	IntID int
	Mood
}

type lineRec struct {
	Eyes, Tongue sql.NullString
	Line
}

type convoRec struct {
	IntID int

	Conversation
}

func newRepository(db *sqlx.DB) (*repository, error) {
	r := repository{db: db}

	stmts := []struct {
		sqlStr string
		stmt   **sqlx.NamedStmt
	}{
		{findMood, &r.findMood},
		{setMood, &r.setMood},
		{deleteMood, &r.deleteMood},
		{insertConvo, &r.insertConvo},
		{getConvo, &r.getConvo},
		{deleteConvo, &r.deleteConvo},
		{findConvoLines, &r.findConvoLines},
		{insertLine, &r.insertLine},
		{getLine, &r.getLine},
		{deleteLine, &r.deleteLine},

		{fmt.Sprintf(listConvos, ">", "ASC"), &r.listConvosAsc},
		{fmt.Sprintf(listConvos, "<", "DESC"), &r.listConvosDesc},
		{fmt.Sprintf(listMoods, ">", "ASC"), &r.listMoodsAsc},
		{fmt.Sprintf(listMoods, "<", "DESC"), &r.listMoodsDesc},
	}

	for _, entry := range stmts {
		prepped, err := db.PrepareNamed(entry.sqlStr)
		*entry.stmt = prepped
		if err != nil {
			return nil, errors.Annotatef(err, "preparing statement %s", entry.sqlStr)
		}
		r.closers = append(r.closers, prepped)
	}

	return &r, nil
}

func (r *repository) Close() error {
	for _, closer := range r.closers {
		if err := closer.Close(); err != nil {
			return errors.Annotatef(err, "closing %s", closer)
		}
	}

	return nil
}

func (r *repository) ListMoods(userID string, args listArgs) ([]Mood, bool, error) {
	sources := make([]func(bool, listArgs) ([]Mood, bool, error), 2)

	userSrc := func(asc bool, args listArgs) ([]Mood, bool, error) {
		return r.listUserMoods(userID, asc, args)
	}

	var asc bool
	if sortAsc(args) {
		asc = true
		sources[0] = userSrc
		sources[1] = r.listBuiltinMoods
	} else {
		asc = false
		sources[1] = userSrc
		sources[0] = r.listBuiltinMoods
	}

	moods, _, err := sources[0](asc, args)
	if err != nil {
		if err != errCursorNotFound {
			return nil, false, err
		}
	} else {
		args.Limit = args.Limit - len(moods)
		args.Before = ""
		args.After = ""

		if len(moods) == args.Limit {
			return moods, true, nil
		}
	}

	moreMoods, hasMore, err := sources[1](asc, args)
	if err != nil {
		return nil, false, err
	}

	for _, mood := range moreMoods {
		moods = append(moods, mood)
	}

	return moods, hasMore, nil

}

func (r *repository) listBuiltinMoods(asc bool, args listArgs) ([]Mood, bool, error) {
	var moods []Mood

	cursor := args.After
	if !asc {
		cursor = args.Before
	}

	limit := args.Limit + 1

	found := args.After == "" && args.Before == ""
	for i := 0; i < len(builtinMoods); i++ {
		var mood *Mood
		if asc {
			mood = builtinMoods[i]
		} else {
			mood = builtinMoods[len(builtinMoods)-1-i]
		}

		if found {
			moods = append(moods, *mood)
			if len(moods) == limit {
				break
			}
		} else if mood.Name == cursor {
			found = true
		}
	}

	if !found {
		return nil, false, errCursorNotFound
	}

	hasMore := len(moods) > args.Limit
	if hasMore {
		moods = moods[:args.Limit]
	}

	return moods, hasMore, nil
}

func (r *repository) listUserMoods(userID string, asc bool, args listArgs) ([]Mood, bool, error) {
	var moods []Mood

	cursor := args.After
	query := r.listMoodsAsc
	if !asc {
		cursor = args.Before
		query = r.listMoodsDesc
	}

	cursorID := -1
	if cursor != "" {
		var mood moodRec

		err := r.findMood.Get(&mood, struct{ UserID, Name string }{userID, cursor})
		if err == sql.ErrNoRows {
			return nil, false, errCursorNotFound
		} else if err != nil {
			return nil, false, errors.Trace(err)
		} else {
			cursorID = mood.IntID
		}
	}

	rows, err := query.Queryx(struct {
		UserID          string
		CursorID, Limit int
	}{userID, cursorID, args.Limit + 1})
	if err != nil {
		return nil, false, errors.Trace(err)
	}
	defer rows.Close()

	for rows.Next() {
		var rec moodRec
		if err := rows.StructScan(&rec); err != nil {
			return nil, false, errors.Trace(err)
		}

		rec.UserDefined = true
		rec.id = rec.IntID
		moods = append(moods, rec.Mood)
	}

	hasMore := len(moods) > args.Limit
	if hasMore {
		moods = moods[:args.Limit]
	}

	return moods, hasMore, nil
}

func (r *repository) GetMood(userID, name string) (*Mood, error) {
	for _, builtin := range builtinMoods {
		if builtin.Name == name {
			// Copy to prevent modifying builtins by the caller
			mood := *builtin
			return &mood, nil
		}
	}

	var rec moodRec
	err := r.findMood.Get(&rec, struct{ UserID, Name string }{userID, name})
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, errors.Trace(err)
	}
	rec.UserDefined = true
	rec.id = rec.IntID

	return &rec.Mood, nil
}

func (r *repository) SetMood(userID string, mood *Mood) error {
	if isBuiltin(mood.Name) {
		return errBuiltinMood
	}

	var id int
	err := r.setMood.QueryRow(struct {
		UserID, Name, Eyes, Tongue string
	}{
		userID, mood.Name, mood.Eyes, mood.Tongue,
	}).Scan(&id)
	if err != nil {
		return errors.Trace(err)
	}
	if id == 0 {
		return errors.Errorf("Unable to update mood %q", mood.Name)
	}

	mood.id = id

	return nil
}

func (r *repository) DeleteMood(userID, name string) error {
	if isBuiltin(name) {
		return errBuiltinMood
	}

	// TODO: test handling error trying to delete a mood with associated lines
	_, err := r.deleteMood.Exec(struct{ UserID, Name string }{userID, name})
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (r *repository) ListConversations(userID string, args listArgs) ([]Conversation, bool, error) {
	var convos []Conversation

	cursor := args.After
	query := r.listConvosAsc
	if !sortAsc(args) {
		cursor = args.Before
		query = r.listConvosDesc
	}

	cursorID := -1
	if cursor != "" {
		var convo convoRec

		err := r.getConvo.Get(&convo, struct{ UserID, PublicID string }{userID, cursor})
		if err == sql.ErrNoRows {
			return nil, false, errCursorNotFound
		} else if err != nil {
			return nil, false, errors.Trace(err)
		} else {
			cursorID = convo.IntID
		}
	}

	rows, err := query.Queryx(struct {
		UserID          string
		CursorID, Limit int
	}{userID, cursorID, args.Limit + 1})
	if err != nil {
		return nil, false, errors.Trace(err)
	}
	defer rows.Close()

	for rows.Next() {
		var rec convoRec
		if err := rows.StructScan(&rec); err != nil {
			return nil, false, errors.Trace(err)
		}

		rec.id = rec.IntID
		convos = append(convos, rec.Conversation)
	}

	hasMore := len(convos) > args.Limit
	if hasMore {
		convos = convos[:args.Limit]
	}

	return convos, hasMore, nil
}

func (r *repository) NewConversation(userID, heading string) (*Conversation, error) {
	var publicID string

	for i := 0; i < maxInsertRetries; i++ {
		rv, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			return nil, errors.Trace(err)
		}
		publicID = convoIDPrefix + strconv.FormatUint(rv.Uint64(), 36)

		var id int
		err = r.insertConvo.QueryRow(struct {
			PublicID, UserID, Heading string
		}{publicID, userID, heading}).Scan(&id)
		if err == nil {
			return &Conversation{
				ID:      publicID,
				Heading: heading,
				id:      id,
			}, nil
		}

		dbErr, ok := err.(*pq.Error)
		if !ok || dbErr.Code != dbErrDupUnique {
			return nil, errors.Trace(err)
		}
	}

	return nil, errors.New("Unable to insert a new, unique conversation")
}

func (r *repository) GetConversation(userID, convoID string) (*Conversation, error) {
	var convo convoRec

	err := r.getConvo.Get(&convo, struct{ UserID, PublicID string }{userID, convoID})
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, errors.Trace(err)
	}

	rows, err := r.findConvoLines.Queryx(struct{ ID int }{convo.IntID})
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rows.Close()

	convo.Lines = make([]Line, 0)
	for rows.Next() {
		var rec lineRec
		if err := rows.StructScan(&rec); err != nil {
			return nil, errors.Trace(err)
		}

		setLineMood(&rec)
		if rec.mood == nil {
			return nil, errors.Errorf("Line %s does not have a valid mood", rec.ID)
		}

		convo.Lines = append(convo.Lines, rec.Line)
	}

	convo.Conversation.id = convo.IntID

	return &convo.Conversation, nil
}

func (r *repository) DeleteConversation(userID, convoID string) error {
	_, err := r.deleteConvo.Exec(struct{ UserID, PublicID string }{userID, convoID})
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (r *repository) InsertLine(userID, convoID string, line *Line) error {
	var publicID string

	var convo convoRec
	err := r.getConvo.Get(&convo, struct{ UserID, PublicID string }{userID, convoID})
	if err != nil {
		return errors.Trace(err)
	}

	for i := 0; i < maxInsertRetries; i++ {
		rv, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			return errors.Trace(err)
		}
		publicID = lineIDPrefix + strconv.FormatUint(rv.Uint64(), 36)

		var moodID sql.NullInt64
		if line.mood.id != 0 {
			moodID.Int64 = int64(line.mood.id)
			moodID.Valid = true
		}

		_, err = r.insertLine.Exec(struct {
			PublicID, Animal, Text, MoodName string
			Think                            bool
			MoodID                           sql.NullInt64
			ConversationID                   int
		}{
			publicID, line.Animal, line.Text, line.MoodName,
			line.Think,
			moodID,
			convo.IntID,
		})
		if err == nil {
			line.ID = publicID
			return nil
		}

		dbErr, ok := err.(*pq.Error)
		if !ok || dbErr.Code != dbErrDupUnique {
			return errors.Trace(err)
		}
	}

	return errors.New("Unable to insert a new, unique line")
}

func (r *repository) GetLine(userID, convoID, lineID string) (*Line, error) {
	var rec lineRec

	err := r.getLine.Get(&rec, struct{ UserID, ConvoID, LineID string }{userID, convoID, lineID})
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, errors.Trace(err)
	}

	setLineMood(&rec)
	if rec.mood == nil {
		return nil, errors.Errorf("Line %s does not have a valid mood", rec.ID)
	}

	return &rec.Line, nil
}

func (r *repository) DeleteLine(userID, convoID, lineID string) error {
	_, err := r.deleteLine.Exec(struct{ UserID, ConvoID, LineID string }{userID, convoID, lineID})
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func setLineMood(rec *lineRec) {
	if rec.Eyes.Valid {
		rec.mood = &Mood{
			Name:        rec.MoodName,
			Eyes:        rec.Eyes.String,
			Tongue:      rec.Tongue.String,
			UserDefined: true,
		}
		return
	}

	for _, mood := range builtinMoods {
		if strings.EqualFold(mood.Name, rec.MoodName) {
			m := *mood
			rec.mood = &m
			return
		}
	}
}

func isBuiltin(name string) bool {
	for _, builtin := range builtinMoods {
		if strings.EqualFold(builtin.Name, name) {
			return true
		}
	}
	return false
}

func sortAsc(args listArgs) bool {
	return args.After != "" || args.Before == ""
}
