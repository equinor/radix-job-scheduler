package v1

import (
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type Handler struct {
	Kube                    *kube.Kube
	Env                     *models.Env
	HandlerApiV2            apiv2.Handler
	RadixDeployJobComponent *radixv1.RadixDeployJobComponent
}
