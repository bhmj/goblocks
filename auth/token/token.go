package token

import (
	"errors"
	"net/http"
)

var errInvalidToken = errors.New("missing or invalid token")

type Auth struct {
	secret string
}

func New(secret string) *Auth {
	return &Auth{secret: secret}
}

func (a *Auth) Authorized(req *http.Request) error {
	headerToken := req.Header.Get("Api-Token")
	if headerToken == a.secret {
		return nil
	}

	return errInvalidToken
}
