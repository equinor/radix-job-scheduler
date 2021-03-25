package utils

import (
	"encoding/json"
	"net/http"

	"github.com/equinor/radix-job-scheduler/models"
)

func JSONResponse(w http.ResponseWriter, result interface{}) {
	body, err := json.Marshal(result)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func StatusResponse(w http.ResponseWriter, status *models.Status) {
	body, err := json.Marshal(status)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status.Code)
	w.Write(body)
}

func WriteResponse(w http.ResponseWriter, statusCode int, response ...string) {
	w.WriteHeader(statusCode)
	for _, responseText := range response {
		w.Write([]byte(responseText))
	}
}

func ErrorResponse(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}
