package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	"goji.io/pat"

	"goji.io"

	"github.com/metcalf/saypi/log"
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

	respond.Data(w, http.StatusOK, res)
}

func (c *Controller) GetUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	id := pat.Param(ctx, "id")
	if id == "" {
		panic("GetUser called without an `id` URL Var")
	}

	if c.getUser(id) != nil {
		w.WriteHeader(204)
	} else {
		respond.NotFound(w, r)
	}
}

// WrapC wraps a handler and only passes requests with valid Bearer authorization.
func (c *Controller) WrapC(inner goji.Handler) goji.Handler {
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			respond.Error(w, http.StatusUnauthorized, usererrors.AuthRequired)
			return
		}

		auth = strings.TrimPrefix(auth, "Bearer ")

		if user := c.getUser(auth); user != nil {
			ctx = context.WithValue(ctx, ctxKey, user)
			log.SetContext(ctx, "user_id", user.ID)
			inner.ServeHTTPC(ctx, w, r)
		} else {
			respond.Error(w, http.StatusUnauthorized, usererrors.AuthInvalid)
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
