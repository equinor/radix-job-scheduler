package controllers

import (
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	models "github.com/equinor/radix-job-scheduler/models/common"
	"github.com/equinor/radix-job-scheduler/utils"
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

	utils.StatusResponse(c.Writer, status)
}
