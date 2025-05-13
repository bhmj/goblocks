package httpreply

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bhmj/goblocks/log"
)

// Replier defines some common and useful functions
type Replier interface {
	ReplyOK(w http.ResponseWriter)                                   // HTTP 200 { "result": "ok" }
	ReplyCreated(w http.ResponseWriter)                              // HTTP 201
	ReplyNoContent(w http.ResponseWriter)                            // HTTP 204
	ReplyError(w http.ResponseWriter, err error, code int)           // HTTP 4xx { "error": "<...>" }
	ReplyJSON(w http.ResponseWriter, data interface{})               // HTTP 200 <serialized object>
	ReplyJSONCode(w http.ResponseWriter, data interface{}, code int) // HTTP xxx <serialized object>
	ReplyString(w http.ResponseWriter, str string)                   // HTTP 200 <text>
}

type replier struct {
	logger log.MetaLogger
}

func NewReplier(logger log.MetaLogger) Replier {
	return &replier{logger: logger}
}

func (r *replier) ReplyOK(w http.ResponseWriter) {
	result := `{ "result": "ok" }`
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(result)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(result)); err != nil {
		r.logger.Error("replyOK", log.Error(err))
	}
}

func (r *replier) ReplyCreated(w http.ResponseWriter) {
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write(nil); err != nil {
		r.logger.Error("replyCreated", log.Error(err))
	}
}

func (r *replier) ReplyNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
	if _, err := w.Write(nil); err != nil {
		r.logger.Error("replyNoContent", log.Error(err))
	}
}

func (r *replier) ReplyError(w http.ResponseWriter, err error, code int) {
	res := map[string]string{"error": err.Error()}
	result, err := json.Marshal(res)
	if err != nil {
		r.logger.Error("replyError", log.Error(err))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(result)))
	w.WriteHeader(code)
	_, err = w.Write(result)
	if err != nil {
		r.logger.Error("replyError", log.Error(err))
	}
}

func (r *replier) ReplyJSONCode(w http.ResponseWriter, data interface{}, code int) {
	result, err := json.Marshal(data)
	if err != nil {
		r.logger.Error("replyJSON", log.Error(err))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(result)))
	w.WriteHeader(code)
	if _, err := w.Write(result); err != nil {
		r.logger.Error("replyJSON", log.Error(err))
	}
}

func (r *replier) ReplyJSON(w http.ResponseWriter, data interface{}) {
	r.ReplyJSONCode(w, data, http.StatusOK)
}

func (r *replier) ReplyString(w http.ResponseWriter, str string) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", strconv.Itoa(len(str)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(str)); err != nil {
		r.logger.Error("replyString", log.Error(err))
	}
}
