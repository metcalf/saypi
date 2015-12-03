package client

import (
	"encoding/json"
	"reflect"

	"github.com/google/go-querystring/query"
	"github.com/metcalf/saypi/say"
)

type ListParams struct {
	After  string `url:"ending_after"`
	Before string `url:"starting_before"`
	Limit  int
}

type listResponse struct {
	Type    string          `json:"type"`
	HasMore bool            `json:"has_more"`
	Cursor  string          `json:"cursor"`
	Data    json.RawMessage `json:"data"`
}

type Iter struct {
	client     *Client
	route      Route
	vars       Vars
	params     ListParams
	hasMore    bool
	values     reflect.Value
	valuesType reflect.Type
	err        error
	cur        reflect.Value
}

// MoodIter is an iterator for lists of Moods. The embedded Iter
// carries methods with it; see its documentation for details.
type MoodIter struct {
	*Iter
}

// Mood returns the most recent Mood visited by a call to Next.
func (it *MoodIter) Mood() say.Mood {
	return it.Current().(say.Mood)
}

// ConversationIter is an iterator for lists of Conversations. The
// embedded Iter carries methods with it; see its documentation for
// details.
type ConversationIter struct {
	*Iter
}

// Conversation returns the most recent Conversation visited by a call
// to Next.
func (it *ConversationIter) Conversation() say.Conversation {
	return it.Current().(say.Conversation)
}

func (it *Iter) getPage() error {
	form, err := query.Values(it.params)
	if err != nil {
		return err
	}

	var listRes listResponse

	_, err = it.client.execute(it.route, it.vars, &form, &listRes)
	if err != nil {
		return err
	}

	it.hasMore = listRes.HasMore
	if it.params.After != "" {
		it.params.After = listRes.Cursor
	} else {
		it.params.Before = listRes.Cursor
	}

	// Create a pointer to a slice value and set it to the slice
	// MakeSlice allocates a value on the stack that is not addressable
	dataPtr := reflect.New(it.valuesType)
	dataPtr.Elem().Set(reflect.MakeSlice(it.valuesType, 0, 0))

	if err := json.Unmarshal(listRes.Data, dataPtr.Interface()); err != nil {
		return err
	}
	it.values = dataPtr.Elem()

	return nil
}

// Next advances the Iter to the next item in the list, which will
// then be available through the Current method. It returns false
// when the iterator stops at the end of the list or an error is
// encountered.
func (it *Iter) Next() bool {
	if it.err != nil {
		return false
	}

	if it.values.Len() == 0 {
		if !it.hasMore {
			return false
		}
		if err := it.getPage(); err != nil {
			it.err = err
			return false
		}
	}
	it.cur = it.values.Index(0)
	it.values = it.values.Slice(1, it.values.Len())
	return true
}

// Current returns the most recent item visited by a call to Next.
func (it *Iter) Current() interface{} {
	return it.cur.Interface()
}

// Err returns the error, if any, that caused the Iter to stop. It
// must be inspected after Next returns false.
func (it *Iter) Err() error {
	return it.err
}

func (c *Client) iter(rt Route, rtVars Vars, params ListParams, item interface{}) *Iter {
	tp := reflect.SliceOf(reflect.TypeOf(item))

	return &Iter{
		client:     c,
		route:      rt,
		hasMore:    true,
		vars:       rtVars,
		params:     params,
		values:     reflect.MakeSlice(tp, 0, 0),
		valuesType: tp,
	}
}
