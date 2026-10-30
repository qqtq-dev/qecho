package response

import (
	"encoding/json"
	"log"
	"net/http"
)

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func write(w http.ResponseWriter, status, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Code: code, Message: message, Data: data}); err != nil {
		log.Printf("write response: %v", err)
	}
}

func OK(w http.ResponseWriter, data any) {
	write(w, http.StatusOK, 0, "ok", data)
}

func BadRequest(w http.ResponseWriter, message string) {
	write(w, http.StatusBadRequest, 40000, message, nil)
}

func Unauthorized(w http.ResponseWriter, code int, message string) {
	write(w, http.StatusUnauthorized, code, message, nil)
}

func TooManyRequests(w http.ResponseWriter, message string) {
	write(w, http.StatusTooManyRequests, 42900, message, nil)
}

func Forbidden(w http.ResponseWriter, message string) {
	write(w, http.StatusForbidden, 40300, message, nil)
}

func NotFound(w http.ResponseWriter, message string) {
	write(w, http.StatusNotFound, 40400, message, nil)
}

func Internal(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	write(w, http.StatusInternalServerError, 50000, "内部错误", nil)
}
