package httpreply

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bhmj/goblocks/log"
)

// Replier defines some common and useful functions
type Replier interface {
	ReplyOK(w http.ResponseWriter) (int, error)                                   // HTTP 200 { "result": "ok" }
	ReplyCreated(w http.ResponseWriter) (int, error)                              // HTTP 201
	ReplyNoContent(w http.ResponseWriter) (int, error)                            // HTTP 204
	ReplyError(w http.ResponseWriter, err error, code int) (int, error)           // HTTP 4xx { "error": "<...>" }
	ReplyJSON(w http.ResponseWriter, data interface{}) (int, error)               // HTTP 200 <serialized object>
	ReplyJSONCode(w http.ResponseWriter, data interface{}, code int) (int, error) // HTTP xxx <serialized object>
	ReplyString(w http.ResponseWriter, str string) (int, error)                   // HTTP 200 <text>
}

type replier struct {
	logger log.MetaLogger
}

func NewReplier(logger log.MetaLogger) Replier {
	return &replier{logger: logger}
}

func (r *replier) ReplyOK(w http.ResponseWriter) (int, error) {
	result := `{ "result": "ok" }`
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(result)))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(result))
	return http.StatusOK, err
}

func (r *replier) ReplyCreated(w http.ResponseWriter) (int, error) {
	w.WriteHeader(http.StatusCreated)
	_, err := w.Write(nil)
	return http.StatusCreated, err
}

func (r *replier) ReplyNoContent(w http.ResponseWriter) (int, error) {
	w.WriteHeader(http.StatusNoContent)
	_, err := w.Write(nil)
	return http.StatusNoContent, err
}

func (r *replier) ReplyError(w http.ResponseWriter, err error, code int) (int, error) {
	res := map[string]string{"error": err.Error()}
	result, err := json.Marshal(res)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(result)))
	w.WriteHeader(code)
	_, err = w.Write(result)
	return code, err
}

func (r *replier) ReplyJSONCode(w http.ResponseWriter, data interface{}, code int) (int, error) {
	result, err := json.Marshal(data)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(result)))
	w.WriteHeader(code)
	_, err = w.Write(result)
	return http.StatusOK, err
}

func (r *replier) ReplyJSON(w http.ResponseWriter, data interface{}) (int, error) {
	return r.ReplyJSONCode(w, data, http.StatusOK)
}

func (r *replier) ReplyString(w http.ResponseWriter, str string) (int, error) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", strconv.Itoa(len(str)))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(str))
	return http.StatusOK, err
}
