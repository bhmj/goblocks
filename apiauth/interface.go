package apiauth

import "net/http"

// Auth is authentication provider
type Auth interface {
	Authorized(req *http.Request) error
}
