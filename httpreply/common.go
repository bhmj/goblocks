package httpreply

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/bhmj/goblocks/log"
)

// Replier defines some common and useful functions
type Replier interface {
	Reply(w http.ResponseWriter, code int, contentType string, content []byte) (int, error)
	ReplyOK(w http.ResponseWriter) (int, error)                             // HTTP 200 { "result": "ok" }
	ReplyCreated(w http.ResponseWriter) (int, error)                        // HTTP 201
	ReplyNoContent(w http.ResponseWriter) (int, error)                      // HTTP 204
	ReplyError(w http.ResponseWriter, err error, code int) (int, error)     // HTTP 4xx { "error": "<...>" }
	ReplyJSON(w http.ResponseWriter, str []byte) (int, error)               // HTTP 200 <serialized object>
	ReplyJSONCode(w http.ResponseWriter, str []byte, code int) (int, error) // HTTP xxx <serialized object>
	ReplyObject(w http.ResponseWriter, data any) (int, error)               // HTTP 200 <unserialized object>
	ReplyObjectCode(w http.ResponseWriter, data any, code int) (int, error) // HTTP xxx <unserialized object>
	ReplyString(w http.ResponseWriter, str string) (int, error)             // HTTP 200 <text>
}

type replier struct {
	logger log.MetaLogger
}

func NewReplier(logger log.MetaLogger) Replier {
	return &replier{logger: logger}
}

func (r *replier) Reply(w http.ResponseWriter, code int, contentType string, content []byte) (int, error) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if len(content) > 0 {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	}
	w.WriteHeader(code)
	_, _ = w.Write(content)
	return code, nil
}

func (r *replier) ReplyOK(w http.ResponseWriter) (int, error) {
	return r.Reply(w, http.StatusOK, "", nil)
}

func (r *replier) ReplyCreated(w http.ResponseWriter) (int, error) {
	return r.Reply(w, http.StatusCreated, "", nil)
}

func (r *replier) ReplyNoContent(w http.ResponseWriter) (int, error) {
	return r.Reply(w, http.StatusNoContent, "", nil)
}

func (r *replier) ReplyError(w http.ResponseWriter, err error, code int) (int, error) {
	return r.Reply(w, code, "application/json", []byte(`{"error":"`+fmt.Sprintf("%s", err)+`"}`))
}

func (r *replier) ReplyJSON(w http.ResponseWriter, str []byte) (int, error) {
	return r.ReplyJSONCode(w, str, http.StatusOK)
}

func (r *replier) ReplyJSONCode(w http.ResponseWriter, str []byte, code int) (int, error) {
	return r.Reply(w, code, "application/json", str)
}

func (r *replier) ReplyObject(w http.ResponseWriter, obj any) (int, error) {
	buf, _ := json.Marshal(obj)
	return r.Reply(w, http.StatusOK, "application/json", buf)
}

func (r *replier) ReplyObjectCode(w http.ResponseWriter, obj any, code int) (int, error) {
	buf, _ := json.Marshal(obj)
	return r.Reply(w, code, "application/json", buf)
}

func (r *replier) ReplyString(w http.ResponseWriter, str string) (int, error) {
	return r.Reply(w, http.StatusOK, "text/plain", []byte(str))
}
