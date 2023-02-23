package controllers

import (
	"net/http"

	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	models "github.com/equinor/radix-job-scheduler/models/common"
	"github.com/equinor/radix-job-scheduler/utils"
	log "github.com/sirupsen/logrus"
)

type ControllerBase struct {
}

func (controller *ControllerBase) HandleError(w http.ResponseWriter, err error) {
	var status *models.Status

	switch t := err.(type) {
	case apiErrors.APIStatus:
		status = t.Status()
	default:
		status = apiErrors.NewFromError(err).Status()
	}

	log.Errorf("failed: %v", err)
	utils.StatusResponse(w, status)
}
