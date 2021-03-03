package utils

import (
	"encoding/json"
	"net/http"
)

func JSONResponse(w http.ResponseWriter, r *http.Request, result interface{}) {
	body, err := json.Marshal(result)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func WriteResponse(w http.ResponseWriter, statusCode int, response ...string) {
	w.WriteHeader(statusCode)
	for _, responseText := range response {
		w.Write([]byte(responseText))
	}
}

func SuccessResponse(w http.ResponseWriter, r *http.Request, statusCode int) {
	w.WriteHeader(statusCode)
}
