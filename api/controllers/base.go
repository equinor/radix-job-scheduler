package controllers

import (
	"encoding/json"
	"net/http"

	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	models "github.com/equinor/radix-job-scheduler/models/common"
	"github.com/gin-gonic/gin"
)

type ControllerBase struct {
}

func (controller *ControllerBase) HandleError(c *gin.Context, err error) {
	_ = c.Error(err)

	var status *models.Status
	switch t := err.(type) {
	case apiErrors.APIStatus:
		status = t.Status()
	default:
		status = apiErrors.NewFromError(err).Status()
	}

	controller.statusResponse(c.Writer, status)
}

func (controller *ControllerBase) statusResponse(w http.ResponseWriter, status *models.Status) {
	body, err := json.Marshal(status)
	if err != nil {
		controller.writeResponse(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status.Code)
	_, _ = w.Write(body)
}

func (controller *ControllerBase) writeResponse(w http.ResponseWriter, statusCode int, response ...string) {
	w.WriteHeader(statusCode)
	for _, responseText := range response {
		_, _ = w.Write([]byte(responseText))
	}
}
