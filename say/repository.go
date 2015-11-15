package say

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"io"
	"math"
	"math/big"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	maxInsertRetries = 16
	convoIDPrefix    = "cv_"
	lineIDPrefix     = "ln_"
	dbErrDupUnique   = "23505"

	listMoods = `
SELECT id, name, eyes, tongue
FROM moods
WHERE user_id = :user_id AND
  (:after = '' OR lower(name) > lower(:after)) AND
  (:before = '' OR lower(name) < lower(:before))
LIMIT :limit + 1
`
	findMood = `
SELECT id, eyes, tongue
FROM moods
WHERE user_id = :user_id AND lower(name) = lower(:name)
`
	deleteMood = `
DELETE FROM moods
WHERE user_id = :user_id AND lower(name) = lower(:name)
`
	// TODO: Racy upsert
	setMood = `
WITH updated as (
  UPDATE moods SET eyes = :eyes, tongue = :tongue
  WHERE user_id = :user_id AND lower(name) = lower(:name)
  RETURNING 1
)
INSERT INTO moods (user_id, name, eyes, tongue)
SELECT :user_id, :name, :eyes, :tongue
WHERE NOT EXISTS (SELECT * FROM updated)
`

	listConvos = `
SELECT public_id, heading
FROM conversations
WHERE user_id = :user_id AND
  (:after = '' OR public_id > :after) AND
  (:before = '' OR public_id < :before)
LIMIT :limit + 1
`
	insertConvo = `
INSERT INTO conversations (public_id, user_id, heading)
SELECT :public_id, :user_id, :heading
`
	getConvo = `
SELECT id, heading FROM conversations
WHERE user_id = :user_id AND public_id = :public_id
`
	deleteConvo = `
DELETE FROM conversations WHERE user_id = :user_id AND public_id = :public_id
`

	findConvoLines = `
SELECT animal, think, text, name as mood_name, eyes, tongue
FROM lines
INNER JOIN moods ON lines.mood_id = moods.id
WHERE conversation_id = :id
`
	insertLine = `
INSERT INTO LINES (public_id, animal, think, text, mood_id, conversation_id)
SELECT :public_id, :animal, :think, :text, :mood_id, :conversation_id
`
	getLine = `
SELECT animal, think, text, name as mood_name, eyes, tongue
FROM lines
INNER JOIN moods ON lines.mood_id = moods.id
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

type repository struct {
	db      *sqlx.DB
	closers []io.Closer

	listMoods, findMood, deleteMood, setMood        *sqlx.NamedStmt
	listConvos, insertConvo, getConvo, deleteConvo  *sqlx.NamedStmt
	findConvoLines, insertLine, getLine, deleteLine *sqlx.NamedStmt
}

type listArgs struct {
	Before, After string
	Limit         int
}

var builtinMoods = []*Mood{
	{0, "borg", "==", "  ", false},
	{0, "dead", "xx", "U ", false},
	{0, "greedy", "$$", "  ", false},
	{0, "stoned", "**", "U ", false},
	{0, "tired", "--", "  ", false},
	{0, "wired", "OO", "  ", false},
	{0, "young", "..", "  ", false},
}

func newRepository(db *sqlx.DB) (*repository, error) {
	r := repository{db: db}

	stmts := map[string]**sqlx.NamedStmt{
		listMoods:      &r.listMoods,
		findMood:       &r.findMood,
		setMood:        &r.setMood,
		deleteMood:     &r.deleteMood,
		listConvos:     &r.listConvos,
		insertConvo:    &r.insertConvo,
		getConvo:       &r.getConvo,
		deleteConvo:    &r.deleteConvo,
		findConvoLines: &r.findConvoLines,
		insertLine:     &r.insertLine,
		getLine:        &r.getLine,
		deleteLine:     &r.deleteLine,
	}

	for sqlStr, stmt := range stmts {
		prepped, err := db.PrepareNamed(sqlStr)
		*stmt = prepped
		if err != nil {
			return nil, err
		}
		r.closers = append(r.closers, prepped)
	}

	return &r, nil
}

func (r *repository) Close() error {
	for _, closer := range r.closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (r *repository) ListMoods(userID string, args *listArgs) ([]Mood, bool, error) {
	var moods []Mood

	err := r.listMoods.Select(&moods, struct {
		UserID string
		*listArgs
	}{userID, args})
	if err != nil {
		return nil, false, err
	}

	for _, m := range moods {
		m.UserDefined = true
	}

	hasMore := len(moods) > args.Limit
	if hasMore {
		moods = moods[:args.Limit]
	}

	for _, m := range builtinMoods {
		moods = append(moods, *m)
	}

	return moods, hasMore, nil
}

func (r *repository) GetMood(userID, name string) (*Mood, error) {
	var m Mood

	for _, builtin := range builtinMoods {
		if builtin.Name == name {
			// Copy to prevent modifying builtins by the caller
			m = *builtin
			return &m, nil
		}
	}

	err := r.findMood.Get(&m, struct{ UserID, Name string }{userID, name})
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	m.UserDefined = true

	return &m, nil
}

func (r *repository) SetMood(userID string, val *Mood) error {
	_, err := r.setMood.Exec(struct {
		UserID, Name, Eyes, Tongue string
	}{
		userID, val.Name, val.Eyes, val.Tongue,
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *repository) DeleteMood(userID, name string) error {
	// TODO: error trying to delete a mood with associated lines
	_, err := r.deleteMood.Exec(struct{ UserID, Name string }{userID, name})
	if err != nil {
		return err
	}

	return nil
}

func (r *repository) ListConversations(userID string, args *listArgs) ([]Conversation, bool, error) {
	var convos []Conversation

	err := r.listConvos.Select(convos, struct {
		UserID string
		*listArgs
	}{userID, args})
	if err != nil {
		return nil, false, err
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
			return nil, err
		}
		publicID = convoIDPrefix + strconv.FormatUint(rv.Uint64(), 36)

		_, err = r.insertConvo.Exec(struct {
			PublicID, UserID, Heading string
		}{publicID, userID, heading})
		if err == nil {
			return &Conversation{
				PublicID: publicID,
				Heading:  heading,
			}, nil
		}

		dbErr, ok := err.(*pq.Error)
		if !ok || dbErr.Code != dbErrDupUnique {
			return nil, err
		}
	}

	return nil, errors.New("Unable to insert a new, unique conversation")
}

func (r *repository) GetConversation(userID, convoID string) (*Conversation, error) {
	var convo Conversation

	err := r.getConvo.Get(&convo, struct{ UserID, PublicID string }{userID, convoID})
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	convo.Lines = make([]Line, 0)

	err = r.findConvoLines.Select(convo.Lines, struct{ ID int }{convo.ID})
	if err != nil {
		return nil, err
	}

	return &convo, nil
}

func (r *repository) DeleteConversation(userID, convoID string) error {
	_, err := r.deleteConvo.Exec(struct{ UserID, PublicID string }{userID, convoID})
	if err != nil {
		return err
	}

	return nil
}

func (r *repository) InsertLine(userID, convoID string, l *Line) error {
	var publicID string

	var convo Conversation
	err := r.getConvo.Get(&convo, struct{ UserID, PublicID string }{userID, convoID})
	if err != nil {
		return err
	}

	for i := 0; i < maxInsertRetries; i++ {
		rv, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			return err
		}
		publicID = lineIDPrefix + strconv.FormatUint(rv.Uint64(), 36)

		_, err = r.insertConvo.Exec(struct {
			PublicID, Animal, Text string
			Think                  bool
			MoodID, CovnersationID int
		}{publicID, l.Animal, l.Text, l.Think, l.Mood.ID, convo.ID})
		if err == nil {
			l.PublicID = publicID
			return nil
		}

		dbErr, ok := err.(*pq.Error)
		if !ok || dbErr.Code != dbErrDupUnique {
			return err
		}
	}

	return errors.New("Unable to insert a new, unique line")
}

func (r *repository) GetLine(userID, convoID, lineID string) (*Line, error) {
	var l Line

	err := r.getLine.Get(&l, struct{ UserID, ConvoID, LineID string }{userID, convoID, lineID})
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &l, nil
}

func (r *repository) DeleteLine(userID, convoID, lineID string) error {
	_, err := r.deleteLine.Exec(struct{ UserID, ConvoID, LineID string }{userID, convoID, lineID})
	if err != nil {
		return err
	}

	return nil
}
