package say

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"goji.io/pat"

	"github.com/jmoiron/sqlx"
	"github.com/metcalf/saypi/auth"
	"github.com/metcalf/saypi/log"

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

	respond(w, res)
}

func (c *Controller) ListMoods(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)

	lArgs, err := getListArgs(r)
	if err != nil {
		// TODO: Potentially unsafe use of error string
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	moods, hasMore, err := c.repo.ListMoods(userID, lArgs)
	if err != nil {
		panic(err)
	}

	respond(w, listRes{
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
		http.NotFound(w, r)
		return
	}

	respond(w, res)
}

func (c *Controller) SetMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	eyes := r.PostFormValue("eyes")
	if !(eyes == "" || utf8.RuneCountInString(eyes) == 2) {
		http.Error(w, "eyes must be a string containing two characters", http.StatusBadRequest)
	}

	tongue := r.PostFormValue("tongue")
	if !(tongue == "" || utf8.RuneCountInString(tongue) == 2) {
		http.Error(w, "tongue must be a string containing two characters", http.StatusBadRequest)
	}

	mood := Mood{
		Name:        name,
		Eyes:        eyes,
		Tongue:      tongue,
		UserDefined: true,
	}

	err := c.repo.SetMood(userID, &mood)
	if err != nil {
		panic(err)
	}

	respond(w, mood)
}

func (c *Controller) DeleteMood(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	name := pat.Param(ctx, "mood")

	if err := c.repo.DeleteMood(userID, name); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) ListConversations(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	lArgs, err := getListArgs(r)
	if err != nil {
		// TODO: Potentially unsafe use of error string
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	convos, hasMore, err := c.repo.ListConversations(userID, lArgs)
	if err != nil {
		panic(err)
	}

	respond(w, listRes{
		HasMore: hasMore,
		Type:    "conversation",
		Data:    convos,
	})
}

func (c *Controller) CreateConversation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)

	heading := r.PostFormValue("heading")
	if cnt := utf8.RuneCountInString(heading); cnt > maxHeadingLength {
		msg := fmt.Sprintf("Param `heading` must be a string of less than %d characters", cnt)
		http.Error(w, msg, http.StatusBadRequest)
	}

	convo, err := c.repo.NewConversation(userID, heading)
	if err != nil {
		panic(err)
	}

	respond(w, convo)
}

func (c *Controller) GetConversation(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := mustUserID(ctx)
	convoID := pat.Param(ctx, "conversation")

	convo, err := c.repo.GetConversation(userID, convoID)
	if err != nil {
		panic(err)
	}
	if convo == nil {
		http.NotFound(w, r)
		return
	}

	for i, Line := range convo.Lines {
		convo.Lines[i].Output, err = c.renderLine(&Line)
		if err != nil {
			panic(err)
		}
	}

	respond(w, convo)
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

	var think bool
	switch r.PostFormValue("think") {
	case "", "false":
		think = false
	case "true":
		think = true
	default:
		msg := "Parameter think must be either 'true' or 'false'"
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	animal := r.PostFormValue("animal")
	if animal == "" {
		animal = "default"
	}
	if _, ok := c.cows[animal]; !ok {
		msg := fmt.Sprintf("Invalid animal name %s", animal)
		http.Error(w, msg, http.StatusBadRequest)
		return
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
		msg := fmt.Sprintf("Invalid mood name %s", moodName)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	line := Line{
		Animal:   animal,
		Think:    think,
		MoodName: moodName,
		Text:     text,
		mood:     mood,
	}

	// TODO: This will panic if you just pass an invalid convo id... bad
	if err := c.repo.InsertLine(userID, convoID, &line); err != nil {
		panic(err)
	}

	line.Output, err = c.renderLine(&line)
	if err != nil {
		panic(err)
	}

	respond(w, line)
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
		http.NotFound(w, r)
		return
	}

	line.Output, err = c.renderLine(line)
	if err != nil {
		panic(err)
	}

	respond(w, line)
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

func getListArgs(r *http.Request) (listArgs, error) {
	res := listArgs{
		After:  r.FormValue("starting_after"),
		Before: r.FormValue("ending_before"),
	}

	if res.After != "" && res.Before != "" {
		return listArgs{}, errors.New("You may not pass `ending_before` if you pass `starting_after`")
	}

	var err error
	limitStr := r.FormValue("limit")
	if limitStr == "" {
		res.Limit = defaultListLimit
	} else {
		res.Limit, err = strconv.Atoi(limitStr)
		if err != nil || res.Limit < 0 || res.Limit > maxListLimit {
			msg := fmt.Sprintf("limit must be a positive integer less than %d", maxListLimit)
			return listArgs{}, errors.New(msg)
		}
	}

	return res, nil
}

func respond(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(data); err == syscall.EPIPE {
		log.Print("respond_broken_pipe", "unable to respond to client", nil)
	} else if err != nil {
		panic(err)
	}
}
