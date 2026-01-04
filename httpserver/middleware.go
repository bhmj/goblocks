package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/bhmj/goblocks/apiauth"
	"github.com/bhmj/goblocks/httpreply"
	"github.com/bhmj/goblocks/log"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// HandlerWithResult is an HTTP handler that returns status code and error
type HandlerWithResult func(w http.ResponseWriter, r *http.Request) (int, error)

type ContextKey string

const ContextRequestID ContextKey = "requestID"

// before router middlewares

func connLimiterMiddleware(next http.Handler, cw *ConnectionWatcher, openConnLimit int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		n := cw.Count()
		if n >= int64(openConnLimit) {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, req)
	}
}

func rateLimiterMiddleware(next http.Handler, limiter *rate.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, req)
	}
}

func authMiddleware(next http.Handler, auth apiauth.Auth) http.HandlerFunc {
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

func panicLoggerMiddleware(next http.Handler, logger log.MetaLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				err, _ := e.(error)
				rid := r.Context().Value(ContextRequestID)
				reqID, _ := rid.(string)
				logger.Error("PANIC", log.String("rid", reqID), log.Stack("stack"))
				_, _ = httpreply.Error(w, err, http.StatusInternalServerError)
				logger.Flush()
			}
		}()
		next.ServeHTTP(w, r)
	}
}

// after router middlewares

func instrumentationMiddleware(handler HandlerWithResult, logger log.MetaLogger, metrics *serviceMetrics, service, endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()

		// get real remote address
		remoteAddr := r.RemoteAddr
		if x, found := r.Header["X-Forwarded-For"]; found {
			remoteAddr = x[0]
		} else if x, found := r.Header["X-Real-Ip"]; found {
			remoteAddr = x[0]
		} else if x, found := r.Header["X-Real-IP"]; found { //nolint:staticcheck
			remoteAddr = x[0]
		}
		r.RemoteAddr = strings.Split(remoteAddr, ":")[0]

		// request ID
		reqID := uuid.New().String()
		// logging
		fields := []log.Field{
			log.String("method", r.Method),
			log.String("uri", r.RequestURI),
			log.String("remote", r.RemoteAddr),
			log.String("rid", reqID),
		}
		contextLogger := logger.With(fields...)
		defer contextLogger.Flush()

		ctx := context.WithValue(r.Context(), log.ContextMetaLogger, contextLogger)
		ctx = context.WithValue(ctx, ContextRequestID, reqID) // used in panic middleware
		contextLogger.Info("start")
		// metrics
		startTime := time.Now()
		code, err := handler(w, r.WithContext(ctx))
		defer metrics.ScoreMethod(service, endpoint, startTime, err)
		contextLogger.Info("finish")
		// errorer
		if err != nil {
			contextLogger.Error("runtime", log.String("rid", reqID), log.Error(err), log.MainMessage())
			_, _ = httpreply.Error(w, err, code)
		}
	}
}
