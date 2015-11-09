package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/metcalf/saypi/mux"
	"golang.org/x/net/context"
)

type Controller struct {
	secret []byte
}

const (
	idLen = 16
)

func New(secret []byte) *Controller {
	return &Controller{secret}
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
	auth, ok := mux.URLVar(ctx, "id")
	if !ok {
		panic("GetUser called without an `id` URL Var")
	}

	if c.getUser(auth[0]) != "" {
		w.WriteHeader(204)
	} else {
		http.NotFound(w, r)
	}
}

func (c *Controller) WrapC(inner mux.HandlerC) mux.HandlerC {
	return mux.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "You must provide a Bearer token in an Authorization header", http.StatusUnauthorized)
			return
		}

		auth = strings.TrimPrefix(auth, "Bearer ")

		if c.getUser(auth) != "" {
			inner.ServeHTTPC(context.WithValue(ctx, "userID", ""), w, r)
		} else {
			http.Error(w, "Invalid authentication string", http.StatusUnauthorized)
		}
	})
}

func (c *Controller) getUser(auth string) string {
	mac := hmac.New(sha256.New, c.secret)

	raw, err := base64.URLEncoding.DecodeString(auth)
	if err != nil {
		return ""
	}
	if len(raw) != idLen+mac.Size() {
		return ""
	}

	id := raw[0:idLen]
	msgMac := raw[idLen:]

	if _, err := mac.Write(id); err != nil {
		panic(err)
	}
	expectMac := mac.Sum(nil)

	if hmac.Equal(msgMac, expectMac) {
		return string(id)
	}
	return ""
}
