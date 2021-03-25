package controllers

import (
	"net/http"

	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/utils"
	log "github.com/sirupsen/logrus"

	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
)

type ControllerBase struct {
}

func (controller *ControllerBase) HandleError(w http.ResponseWriter, err error) {
	var status *models.Status

	switch t := err.(type) {
	case jobErrors.APIStatus:
		status = t.Status()
	default:
		status = jobErrors.NewFromError(err).Status()
	}

	log.Errorf("failed: %v", err)
	utils.StatusResponse(w, status)

}
