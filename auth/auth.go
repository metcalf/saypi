package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"goji.io/pat"
	"goji.io/pattern"

	"goji.io"

	"github.com/metcalf/saypi/reqlog"
	"github.com/metcalf/saypi/respond"
	"github.com/metcalf/saypi/usererrors"
	"golang.org/x/net/context"
)

type Controller struct {
	secret []byte
}

const (
	idLen  = 16
	ctxKey = "auth.User"
)

func New(secret []byte) (*Controller, error) {
	return &Controller{secret}, nil
}

func (c *Controller) CreateUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	id := make([]byte, idLen)
	if _, err := rand.Read(id); err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	mac := hmac.New(sha256.New, c.secret)
	if _, err := mac.Write(id); err != nil {
		respond.InternalError(ctx, w, err)
		return
	}

	msg := mac.Sum(id)

	res := struct {
		ID string `json:"id"`
	}{base64.URLEncoding.EncodeToString(msg)}

	respond.Data(ctx, w, http.StatusOK, res)
}

func (c *Controller) GetUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	id := pat.Param(ctx, "id")
	if id == "" {
		respond.InternalError(ctx, w, errors.New("GetUser called without an `id` URL Var"))
		return
	}

	if c.getUser(id) != nil {
		w.WriteHeader(204)
	} else {
		respond.NotFound(ctx, w, r)
	}
}

// WrapC wraps a handler and only passes requests with valid Bearer authorization.
func (c *Controller) WrapC(inner goji.Handler) goji.Handler {
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			respond.UserError(ctx, w, http.StatusUnauthorized, BearerAuthRequired{})
			return
		}

		auth = strings.TrimPrefix(auth, "Bearer ")

		if user := c.getUser(auth); user != nil {
			ctx = context.WithValue(ctx, ctxKey, user)
			reqlog.SetContext(ctx, "user_id", user.ID)
			inner.ServeHTTPC(ctx, w, r)
		} else {
			respond.UserError(ctx, w, http.StatusUnauthorized, usererrors.AuthInvalid{})
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

func (u *User) Vars() map[pattern.Variable]string {
	return map[pattern.Variable]string{
		"id": u.ID,
	}
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
