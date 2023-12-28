package utils

import (
	"encoding/json"
	"net/http"

	models "github.com/equinor/radix-job-scheduler/models/common"
)

func JSONResponse(w http.ResponseWriter, result interface{}) {
	body, err := json.Marshal(result)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func StatusResponse(w http.ResponseWriter, status *models.Status) {
	body, err := json.Marshal(status)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status.Code)
	_, _ = w.Write(body)
}

func WriteResponse(w http.ResponseWriter, statusCode int, response ...string) {
	w.WriteHeader(statusCode)
	for _, responseText := range response {
		_, _ = w.Write([]byte(responseText))
	}
}

func ErrorResponse(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(err.Error()))
}
