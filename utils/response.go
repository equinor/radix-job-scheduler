package utils

import (
	"encoding/json"
	"net/http"
)

func JSONResult(w http.ResponseWriter, r *http.Request, result interface{}) {
	body, err := json.Marshal(result)
	if err != nil {
		ErrorResult(w, r, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func ErrorResult(w http.ResponseWriter, r *http.Request, statusCode int) {
	w.WriteHeader(statusCode)
}
