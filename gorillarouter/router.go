package gorillarouter

import (
	"net/http"

	"github.com/gorilla/mux"
)

// GorillaRouter wraps a gorilla/mux router.
// It allows to use variables and regexps in URL path and query values.
type GorillaRouter struct {
	router *mux.Router
}

func (gr *GorillaRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gr.router.ServeHTTP(w, r)
}

func (gr *GorillaRouter) HandleFunc(method, pattern string, handler func(http.ResponseWriter, *http.Request)) {
	gr.router.HandleFunc(pattern, handler).Methods(method)
}

func New() *GorillaRouter {
	return &GorillaRouter{router: mux.NewRouter()}
}
