package httpserver

import (
	"net/http"

	"github.com/bhmj/goblocks/apiauth"
	"golang.org/x/time/rate"
)

func AuthenticationMiddleware(next http.Handler, auth apiauth.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if auth != nil {
			if err := auth.Authorized(r); err != nil {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	}
}

func RateLimiterMiddleware(next http.Handler, limiter *rate.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, req)
	}
}

func ConnLimiterMiddleware(next http.Handler, cw *ConnectionWatcher, openConnLimit int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		n := cw.Count()
		if n >= int64(openConnLimit) {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, req)
	}
}
