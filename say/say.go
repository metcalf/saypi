package say

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"goji.io/pat"
	"goji.io/pattern"

	"github.com/gorilla/schema"
	"github.com/jmoiron/sqlx"
	"github.com/metcalf/saypi/auth"
	"github.com/metcalf/saypi/respond"
	"github.com/metcalf/saypi/usererrors"

	"golang.org/x/net/context"
)

const (
	defaultListLimit = 10
	maxListLimit     = 100
	maxHeadingLength = 60
	maxTextLength    = 1024
)

type Controller struct {
	repo *repository
	cows map[string]*cow
}

type getAnimalsRes struct {
	Animals []string `json:"animals"`
}

type Mood struct {
	Name        string `json:"name" url:"-"`
	Eyes        string `json:"eyes" url:"eyes"`
	Tongue      string `json:"tongue" url:"tongue"`
	UserDefined bool   `json:"user_defined" url:"-"`

	id int
}

func (m *Mood) Vars() map[pattern.Variable]string {
	return map[pattern.Variable]string{
		"mood": m.Name,
	}
}

type Line struct {
	ID       string `json:"id" url:"-"`
	Animal   string `json:"animal" url:"animal"`
	Think    bool   `json:"think" url:"think"`
	MoodName string `json:"mood" url:"mood"`
	Text     string `json:"text" url:"text"`
	Output   string `json:"output" url:"-"`

	mood *Mood
}

type Conversation struct {
	ID      string `json:"id",url:"-"`
	Heading string `json:"heading" url:"heading"`
	Lines   []Line `json:"lines,omitempty"`

	id int
}

func (c *Conversation) Vars() map[pattern.Variable]string {
	return map[pattern.Variable]string{
		"conversation": c.ID,
	}
}

type listRes struct {
	Type    string      `json:"type"`
	HasMore bool        `json:"has_more"`
	Cursor  string      `json:"cursor"`
	Data    interface{} `json:"data"`
}

var decoder *schema.Decoder

func init() {
	decoder = schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)
	decoder.SetAliasTag("url") // For compatibility with go-querystring
}

func New(db *sqlx.DB) (*Controller, error) {
	var ctrl Controller
	var err error

	ctrl.repo, err = newRepository(db)
	if err != nil {
		return nil, err
	}

	animals := listAnimals()
	ctrl.cows = make(map[string]*cow, len(animals))
	for _, name := range animals {
		ctrl.cows[name], err = newCow(name)
		if err != nil {
			return nil, err
		}
	}

	return &ctrl, nil
}

func (c *Controller) Close() error {
	if err := c.repo.Close(); err != nil {
		return err
	}

	return nil
}

func (c *Controller) GetAnimals(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	animals := make([]string, 0, len(c.cows))
	for name := range c.cows {
		animals = append(animals, name)
	}
	res := getAnimalsRes{animals}

	respond.Data(ctx, w, http.StatusOK, res)
}

func (c *Controller) ListMoods(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)

	lArgs, uerr := getListArgs(r)
	if uerr != nil {
		respond.UserError(ctx, w, http.StatusBadRequest, uerr)
		return
	}

	moods, hasMore, err := c.repo.ListMoods(userID, lArgs)
	if err == errCursorNotFound {
		respondCursorNotFound(ctx, w, lArgs)
		return
	} else if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	respond.Data(ctx, w, http.StatusOK, listRes{
		Cursor:  moods[len(moods)-1].Name,
		Type:    "mood",
		HasMore: hasMore,
		Data:    moods,
	})
}

func (c *Controller) GetMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	res, err := c.repo.GetMood(userID, name)
	if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}
	if res == nil {
		respond.NotFound(ctx, w, r)
		return
	}

	respond.Data(ctx, w, http.StatusOK, res)
}

func (c *Controller) SetMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	var mood Mood
	r.ParseForm()
	err := decoder.Decode(&mood, r.PostForm)
	if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	mood.Eyes = strings.Replace(mood.Eyes, "\x00", "", -1)
	mood.Tongue = strings.Replace(mood.Tongue, "\x00", "", -1)

	var uerr usererrors.InvalidParams
	if !(mood.Eyes == "" || utf8.RuneCountInString(mood.Eyes) == 2) {
		uerr = append(uerr, usererrors.InvalidParamsEntry{
			Params:  []string{"eyes"},
			Message: "must be a string containing two characters",
		})
	}

	if !(mood.Tongue == "" || utf8.RuneCountInString(mood.Tongue) == 2) {
		uerr = append(uerr, usererrors.InvalidParamsEntry{
			Params:  []string{"tongue"},
			Message: "must be a string containing two characters",
		})
	}

	if uerr != nil {
		respond.UserError(ctx, w, http.StatusBadRequest, uerr)
		return
	}

	mood.Name = name
	mood.UserDefined = true

	err = c.repo.SetMood(userID, &mood)
	if err == errBuiltinMood {
		respond.UserError(ctx, w, http.StatusBadRequest, usererrors.ActionNotAllowed{
			Action: fmt.Sprintf("update built-in mood %s", name),
		})
		return
	} else if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	respond.Data(ctx, w, http.StatusOK, mood)
}

func (c *Controller) DeleteMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	if err := c.repo.DeleteMood(userID, name); err == errBuiltinMood {
		respond.UserError(ctx, w, http.StatusBadRequest, usererrors.ActionNotAllowed{
			Action: fmt.Sprintf("delete built-in mood %s", name),
		})
		return
	} else if err == errRecordNotFound {
		respond.NotFound(ctx, w, r)
	} else if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) ListConversations(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	lArgs, uerr := getListArgs(r)
	if uerr != nil {
		respond.UserError(ctx, w, http.StatusBadRequest, uerr)
		return
	}

	convos, hasMore, err := c.repo.ListConversations(userID, lArgs)
	if err == errCursorNotFound {
		respondCursorNotFound(ctx, w, lArgs)
		return
	} else if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	respond.Data(ctx, w, http.StatusOK, listRes{
		Cursor:  convos[len(convos)-1].ID,
		Type:    "conversation",
		HasMore: hasMore,
		Data:    convos,
	})
}

func (c *Controller) CreateConversation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)

	heading := strings.Replace(r.PostFormValue("heading"), "\x00", "", -1)
	if cnt := utf8.RuneCountInString(heading); cnt > maxHeadingLength {
		respond.UserError(ctx, w, http.StatusBadRequest, usererrors.InvalidParams{{
			Params:  []string{"heading"},
			Message: fmt.Sprintf("must be a string of less than %d characters", maxHeadingLength),
		}})
		return
	}

	convo, err := c.repo.NewConversation(userID, heading)
	if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	respond.Data(ctx, w, http.StatusOK, convo)
}

func (c *Controller) GetConversation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")

	convo, err := c.repo.GetConversation(userID, convoID)
	if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}
	if convo == nil {
		respond.NotFound(ctx, w, r)
		return
	}

	for i, Line := range convo.Lines {
		convo.Lines[i].Output, err = c.renderLine(&Line)
		if err != nil {
			respond.InternalError(ctx, w, err)
			return
		}
	}

	respond.Data(ctx, w, http.StatusOK, convo)
}

func (c *Controller) DeleteConversation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")

	if err := c.repo.DeleteConversation(userID, convoID); err == errRecordNotFound {
		respond.NotFound(ctx, w, r)
	} else if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TODO: use gorilla schema here
func (c *Controller) CreateLine(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")

	var uerr usererrors.InvalidParams

	var think bool
	switch r.PostFormValue("think") {
	case "", "false":
		think = false
	case "true":
		think = true
	default:
		uerr = append(uerr, usererrors.InvalidParamsEntry{
			Params:  []string{"think"},
			Message: "must be either 'true' or 'false'",
		})
	}

	animal := r.PostFormValue("animal")
	if animal == "" {
		animal = "default"
	}
	if _, ok := c.cows[animal]; !ok {
		uerr = append(uerr, usererrors.InvalidParamsEntry{
			Params:  []string{"animal"},
			Message: fmt.Sprintf("%q does not exist", animal),
		})
	}

	text := strings.Replace(r.PostFormValue("text"), "\x00", "", -1)
	if cnt := utf8.RuneCountInString(text); cnt > maxTextLength {
		respond.UserError(ctx, w, http.StatusBadRequest, usererrors.InvalidParams{{
			Params:  []string{"text"},
			Message: fmt.Sprintf("must be a string of less than %d characters", maxTextLength),
		}})
		return
	}

	moodName := strings.Replace(r.PostFormValue("mood"), "\x00", "", -1)
	if moodName == "" {
		moodName = "default"
	}

	mood, err := c.repo.GetMood(userID, moodName)
	if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}
	if mood == nil {
		uerr = append(uerr, usererrors.InvalidParamsEntry{
			Params:  []string{"mood"},
			Message: fmt.Sprintf("%q does not exist", moodName),
		})
	}

	if uerr != nil {
		respond.UserError(ctx, w, http.StatusBadRequest, uerr)
		return
	}

	line := Line{
		Animal:   animal,
		Think:    think,
		MoodName: moodName,
		Text:     text,
		mood:     mood,
	}

	if err := c.repo.InsertLine(userID, convoID, &line); err == sql.ErrNoRows {
		// The underlying conversation does not exist
		respond.NotFound(ctx, w, r)
	} else if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	line.Output, err = c.renderLine(&line)
	if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	respond.Data(ctx, w, http.StatusOK, line)
}

func (c *Controller) GetLine(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")
	lineID := pat.Param(ctx, "line")

	line, err := c.repo.GetLine(userID, convoID, lineID)
	if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}
	if line == nil {
		respond.NotFound(ctx, w, r)
		return
	}

	line.Output, err = c.renderLine(line)
	if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	respond.Data(ctx, w, http.StatusOK, line)
}

func (c *Controller) DeleteLine(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")
	lineID := pat.Param(ctx, "line")

	if err := c.repo.DeleteLine(userID, convoID, lineID); err == errRecordNotFound {
		respond.NotFound(ctx, w, r)
	} else if err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) renderLine(line *Line) (string, error) {
	cow, ok := c.cows[line.Animal]
	if !ok {
		return "", fmt.Errorf("Unknown animal %q", line.Animal)
	}

	return cow.Say(line.Text, line.mood.Eyes, line.mood.Tongue, line.Think)
}

func mustUserID(ctx context.Context) string {
	// get the var and user id
	user, ok := auth.FromContext(ctx)
	if !ok {
		panic("Missing user in request context")
	}

	return user.ID
}

func getListArgs(r *http.Request) (listArgs, usererrors.UserError) {
	res := listArgs{
		After:  r.FormValue("starting_after"),
		Before: r.FormValue("ending_before"),
	}

	if res.After != "" && res.Before != "" {
		return listArgs{}, usererrors.InvalidParams{{
			Params:  []string{"starting_after", "ending_before"},
			Message: "you may not provide multiple cursor parameters",
		}}
	}

	var err error
	limitStr := r.FormValue("limit")
	if limitStr == "" {
		res.Limit = defaultListLimit
	} else {
		res.Limit, err = strconv.Atoi(limitStr)
		if err != nil || res.Limit < 0 || res.Limit > maxListLimit {
			return listArgs{}, usererrors.InvalidParams{{
				Params:  []string{"limit"},
				Message: fmt.Sprintf("must be a positive integer less than %d", maxListLimit),
			}}
		}
	}

	return res, nil
}

func respondCursorNotFound(ctx context.Context, w http.ResponseWriter, args listArgs) {
	var cursorParam string
	if args.After == "" {
		cursorParam = "ending_before"
	} else {
		cursorParam = "starting_after"
	}

	respond.UserError(ctx, w, http.StatusBadRequest, usererrors.InvalidParams{{
		Params:  []string{cursorParam},
		Message: "must refer to an existing object",
	}})
}
