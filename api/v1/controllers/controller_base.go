package controllers

import (
	"github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models/common"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/gin-gonic/gin"
)

type Route struct {
	Path    string
	Method  string
	Handler gin.HandlerFunc
}

type Controller interface {
	GetRoutes() []Route
}

type ControllerBase struct {
}

func (controller *ControllerBase) HandleError(c *gin.Context, err error) {
	_ = c.Error(err)

	var status *common.Status
	switch t := err.(type) {
	case errors.APIStatus:
		status = t.Status()
	default:
		status = errors.NewFromError(err).Status()
	}

	utils.StatusResponse(c.Writer, status)
}
