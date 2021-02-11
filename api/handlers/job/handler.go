package job

import (
	"context"

	"github.com/equinor/radix-job-scheduler/models"
)

type JobHandler interface {
	GetJobs(ctx context.Context)
	GetJob(ctx context.Context)
	CreateJob(ctx context.Context)
	DeleteJob(ctx context.Context)
}

type jobHandler struct {
	kubeUtil models.KubeUtil
}

func New(kubeUtil models.KubeUtil) JobHandler {
	return &jobHandler{
		kubeUtil: kubeUtil,
	}
}

func (jh *jobHandler) GetJobs(ctx context.Context) {

}

func (jh *jobHandler) GetJob(ctx context.Context) {

}

func (jh *jobHandler) CreateJob(ctx context.Context) {

}

func (jh *jobHandler) DeleteJob(ctx context.Context) {

}
