package job

import (
	"context"

	"github.com/equinor/radix-job-scheduler/models"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Handler interface {
	GetJobs(ctx context.Context) ([]models.Job, error)
	GetJob(ctx context.Context)
	CreateJob(ctx context.Context)
	DeleteJob(ctx context.Context)
}

type jobHandler struct {
	kubeUtil models.KubeUtil
}

func New(kubeUtil models.KubeUtil) Handler {
	return &jobHandler{
		kubeUtil: kubeUtil,
	}
}

func (jh *jobHandler) GetJobs(ctx context.Context) ([]models.Job, error) {
	kubeClient := jh.kubeUtil.Client()
	kubeJobs, err := kubeClient.BatchV1().Jobs("echo-nils-app").List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	jobs := make([]models.Job, len(kubeJobs.Items))

	for idx, k8sJob := range kubeJobs.Items {
		jobs[idx] = *models.GetJobFromK8sJob(&k8sJob)
	}

	return jobs, nil
}

func (jh *jobHandler) GetJob(ctx context.Context) {

}

func (jh *jobHandler) CreateJob(ctx context.Context) {

}

func (jh *jobHandler) DeleteJob(ctx context.Context) {

}
