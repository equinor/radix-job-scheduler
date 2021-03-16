package controllers

import (
	"github.com/equinor/radix-job-scheduler/utils"
	log "github.com/sirupsen/logrus"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"net/http"
)

type ControllerBase struct {
}

func (controller *ControllerBase) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	statusCode := http.StatusInternalServerError
	if apiErrors.IsNotFound(err) {
		statusCode = http.StatusNotFound
	}
	log.Errorf("failed: %v", err)
	utils.WriteResponse(w, statusCode, err.Error())
}
