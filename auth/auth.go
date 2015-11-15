package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/metcalf/saypi/log"
	"github.com/metcalf/saypi/mux"
	"golang.org/x/net/context"
)

type Controller struct {
	secret []byte
}

// TODO: I don't really want these values polluting the pulic interface.
var TestSecret = []byte("shhh")

const (
	idLen  = 16
	ctxKey = "auth.User"

	// TODO: I don't really want these values polluting the pulic interface.
	TestValidUser = "DUtRr7IlHC-fd2wH8tfX_iLM8p8-3yeF4MTbc89B1lt41mk17sOlb6sg3JF_z6Sv"
	// This should be the same length as the validUser so it parses the same way
	TestInvalidUser = "ABCDr7IlHC-fd2wH8tfX_iLM8p8-3yeF4MTbc89B1lt41mk17sOlb6sg3JF_z6Sv"
)

func New(secret []byte) (*Controller, error) {
	return &Controller{secret}, nil
}

func (c *Controller) CreateUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	id := make([]byte, idLen)
	if _, err := rand.Read(id); err != nil {
		panic(err)
	}

	mac := hmac.New(sha256.New, c.secret)
	if _, err := mac.Write(id); err != nil {
		panic(err)
	}

	msg := mac.Sum(id)

	res := struct {
		ID string `json:"id"`
	}{base64.URLEncoding.EncodeToString(msg)}

	if err := json.NewEncoder(w).Encode(res); err != nil {
		// TODO: This shouldn't panic but handle some errors
		panic(err)
	}
}

func (c *Controller) GetUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	match := mux.FromContext(ctx)
	if match == nil {
		panic("GetUser called without an `id` URL Var")
	}
	auth := match.Vars().Get("id")

	if c.getUser(auth) != nil {
		w.WriteHeader(204)
	} else {
		http.NotFound(w, r)
	}
}

// WrapC wraps a handler and only passes requests with valid Bearer authorization.
func (c *Controller) WrapC(inner mux.HandlerC) mux.HandlerC {
	return mux.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "You must provide a Bearer token in an Authorization header", http.StatusUnauthorized)
			return
		}

		auth = strings.TrimPrefix(auth, "Bearer ")

		if user := c.getUser(auth); user != nil {
			ctx = context.WithValue(ctx, ctxKey, user)
			log.SetContext(ctx, "user_id", user.ID)
			inner.ServeHTTPC(ctx, w, r)
		} else {
			http.Error(w, "Invalid authentication string", http.StatusUnauthorized)
		}
	})
}

// FromContext extracts the User from the context, if present.
func FromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(ctxKey).(*User)
	if !ok {
		return User{}, false
	}
	return *user, true
}

// User represents the user that authenticates.
type User struct {
	ID string
}

func (c *Controller) getUser(auth string) *User {
	mac := hmac.New(sha256.New, c.secret)

	raw, err := base64.URLEncoding.DecodeString(auth)
	if err != nil {
		return nil
	}
	if len(raw) != idLen+mac.Size() {
		return nil
	}

	id := raw[0:idLen]
	msgMac := raw[idLen:]

	if _, err := mac.Write(id); err != nil {
		panic(err)
	}
	expectMac := mac.Sum(nil)

	if hmac.Equal(msgMac, expectMac) {
		return &User{base64.URLEncoding.EncodeToString(id)}
	}
	return nil
}
