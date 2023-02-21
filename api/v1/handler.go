package v1

import (
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
)

type Handler struct {
	Kube         *kube.Kube
	Env          *models.Env
	HandlerApiV2 apiv2.Handler
}
