package say

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"goji.io/pat"

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
)

type Controller struct {
	repo *repository
	cows map[string]*cow
}

type getAnimalsRes struct {
	Animals []string `json:"animals"`
}

type Mood struct {
	Name        string `json:"name"`
	Eyes        string `json:"eyes"`
	Tongue      string `json:"tongue"`
	UserDefined bool   `json:"user_defined"`

	id int
}

type Line struct {
	ID       string `json:"id"`
	Animal   string `json:"animal"`
	Think    bool   `json:"think"`
	MoodName string `json:"mood"`
	Text     string `json:"text"`
	Output   string `json:"output"`

	mood *Mood
}

type Conversation struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
	Lines   []Line `json:"lines,omitempty"`

	id int
}

type listRes struct {
	Type    string      `json:"type"`
	HasMore bool        `json:"has_more"`
	Data    interface{} `json:"data"`
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

func (c *Controller) GetAnimals(w http.ResponseWriter, r *http.Request) {
	animals := make([]string, 0, len(c.cows))
	for name := range c.cows {
		animals = append(animals, name)
	}
	res := getAnimalsRes{animals}

	respond.Data(w, http.StatusOK, res)
}

func (c *Controller) ListMoods(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)

	lArgs, uerr := getListArgs(r)
	if uerr != nil {
		respond.Error(w, http.StatusBadRequest, uerr)
		return
	}

	moods, hasMore, err := c.repo.ListMoods(userID, lArgs)
	if err == errCursorNotFound {
		respondCursorNotFound(w, lArgs)
		return
	} else if err != nil {
		panic(err)
	}

	respond.Data(w, http.StatusOK, listRes{
		HasMore: hasMore,
		Type:    "mood",
		Data:    moods,
	})
}

func (c *Controller) GetMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	res, err := c.repo.GetMood(userID, name)
	if err != nil {
		panic(err)
	}
	if res == nil {
		respond.NotFound(w, r)
		return
	}

	respond.Data(w, http.StatusOK, res)
}

func (c *Controller) SetMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	var uerr usererrors.InvalidParams

	eyes := r.PostFormValue("eyes")
	if !(eyes == "" || utf8.RuneCountInString(eyes) == 2) {
		uerr = append(uerr, usererrors.InvalidParams{{
			Params:  []string{"eyes"},
			Message: "must be a string containing two characters",
		}}[0])
	}

	tongue := r.PostFormValue("tongue")
	if !(tongue == "" || utf8.RuneCountInString(tongue) == 2) {
		uerr = append(uerr, usererrors.InvalidParams{{
			Params:  []string{"tongue"},
			Message: "must be a string containing two characters",
		}}[0])
	}

	if uerr != nil {
		respond.Error(w, http.StatusBadRequest, uerr)
		return
	}

	mood := Mood{
		Name:        name,
		Eyes:        eyes,
		Tongue:      tongue,
		UserDefined: true,
	}

	err := c.repo.SetMood(userID, &mood)
	if err == errBuiltinMood {
		respond.Error(w, http.StatusBadRequest, usererrors.ActionNotAllowed{
			Action: fmt.Sprintf("update built-in mood %s", name),
		})
		return
	} else if err != nil {
		panic(err)
	}

	respond.Data(w, http.StatusOK, mood)
}

func (c *Controller) DeleteMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	if err := c.repo.DeleteMood(userID, name); err == errBuiltinMood {
		respond.Error(w, http.StatusBadRequest, usererrors.ActionNotAllowed{
			Action: fmt.Sprintf("delete built-in mood %s", name),
		})
		return
	} else if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) ListConversations(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	lArgs, uerr := getListArgs(r)
	if uerr != nil {
		respond.Error(w, http.StatusBadRequest, uerr)
		return
	}

	convos, hasMore, err := c.repo.ListConversations(userID, lArgs)
	if err == errCursorNotFound {
		respondCursorNotFound(w, lArgs)
		return
	} else if err != nil {
		panic(err)
	}

	respond.Data(w, http.StatusOK, listRes{
		HasMore: hasMore,
		Type:    "conversation",
		Data:    convos,
	})
}

func (c *Controller) CreateConversation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)

	heading := r.PostFormValue("heading")
	if cnt := utf8.RuneCountInString(heading); cnt > maxHeadingLength {
		respond.Error(w, http.StatusBadRequest, usererrors.InvalidParams{{
			Params:  []string{"heading"},
			Message: fmt.Sprintf("must be a string of less than %d characters", maxHeadingLength),
		}})
		return
	}

	convo, err := c.repo.NewConversation(userID, heading)
	if err != nil {
		panic(err)
	}

	respond.Data(w, http.StatusOK, convo)
}

func (c *Controller) GetConversation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")

	convo, err := c.repo.GetConversation(userID, convoID)
	if err != nil {
		panic(err)
	}
	if convo == nil {
		respond.NotFound(w, r)
		return
	}

	for i, Line := range convo.Lines {
		convo.Lines[i].Output, err = c.renderLine(&Line)
		if err != nil {
			panic(err)
		}
	}

	respond.Data(w, http.StatusOK, convo)
}

func (c *Controller) DeleteConversation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")

	if err := c.repo.DeleteConversation(userID, convoID); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

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
		uerr = append(uerr, usererrors.InvalidParams{{
			Params:  []string{"think"},
			Message: "must be either 'true' or 'false'",
		}}[0])
	}

	animal := r.PostFormValue("animal")
	if animal == "" {
		animal = "default"
	}
	if _, ok := c.cows[animal]; !ok {
		uerr = append(uerr, usererrors.InvalidParams{{
			Params:  []string{"animal"},
			Message: fmt.Sprintf("%q does not exist", animal),
		}}[0])
	}

	// Sanitize null bytes for the database
	moodName := strings.Replace(r.PostFormValue("mood"), "\x00", "", -1)
	text := strings.Replace(r.PostFormValue("text"), "\x00", "", -1)

	if moodName == "" {
		moodName = "default"
	}

	mood, err := c.repo.GetMood(userID, moodName)
	if err != nil {
		panic(err)
	}
	if mood == nil {
		uerr = append(uerr, usererrors.InvalidParams{{
			Params:  []string{"mood"},
			Message: fmt.Sprintf("%q does not exist", moodName),
		}}[0])
	}

	if uerr != nil {
		respond.Error(w, http.StatusBadRequest, uerr)
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
		respond.NotFound(w, r)
	} else if err != nil {
		panic(err)
	}

	line.Output, err = c.renderLine(&line)
	if err != nil {
		panic(err)
	}

	respond.Data(w, http.StatusOK, line)
}

func (c *Controller) GetLine(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")
	lineID := pat.Param(ctx, "line")

	line, err := c.repo.GetLine(userID, convoID, lineID)
	if err != nil {
		panic(err)
	}
	if line == nil {
		respond.NotFound(w, r)
		return
	}

	line.Output, err = c.renderLine(line)
	if err != nil {
		panic(err)
	}

	respond.Data(w, http.StatusOK, line)
}

func (c *Controller) DeleteLine(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")
	lineID := pat.Param(ctx, "line")

	if err := c.repo.DeleteLine(userID, convoID, lineID); err != nil {
		panic(err)
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

func respondCursorNotFound(w http.ResponseWriter, args listArgs) {
	var cursorParam string
	if args.After == "" {
		cursorParam = "ending_before"
	} else {
		cursorParam = "starting_after"
	}

	respond.Error(w, http.StatusBadRequest, usererrors.InvalidParams{{
		Params:  []string{cursorParam},
		Message: "must refer to an existing object",
	}})
}
