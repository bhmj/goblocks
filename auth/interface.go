package auth

import "net/http"

// Authentication provider.
type Auth interface {
	Authorized(req *http.Request) error
}
