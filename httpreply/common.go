package httpreply

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Replier defines some common and useful functions

func Reply(w http.ResponseWriter, code int, contentType string, content []byte) (int, error) {
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

func OK(w http.ResponseWriter) (int, error) {
	return Reply(w, http.StatusOK, "", nil)
}

func Created(w http.ResponseWriter) (int, error) {
	return Reply(w, http.StatusCreated, "", nil)
}

func NoContent(w http.ResponseWriter) (int, error) {
	return Reply(w, http.StatusNoContent, "", nil)
}

func Error(w http.ResponseWriter, err error, code int) (int, error) {
	return Reply(w, code, "application/json", []byte(`{"error":"`+fmt.Sprintf("%s", err)+`"}`))
}

func JSON(w http.ResponseWriter, str []byte) (int, error) {
	return JSONCode(w, str, http.StatusOK)
}

func JSONCode(w http.ResponseWriter, str []byte, code int) (int, error) {
	return Reply(w, code, "application/json", str)
}

func Object(w http.ResponseWriter, obj any) (int, error) {
	buf, _ := json.Marshal(obj) //nolint:errchkjson
	return Reply(w, http.StatusOK, "application/json", buf)
}

func ObjectCode(w http.ResponseWriter, obj any, code int) (int, error) {
	buf, _ := json.Marshal(obj) //nolint:errchkjson
	return Reply(w, code, "application/json", buf)
}

func String(w http.ResponseWriter, str string) (int, error) {
	return Reply(w, http.StatusOK, "text/plain", []byte(str))
}
