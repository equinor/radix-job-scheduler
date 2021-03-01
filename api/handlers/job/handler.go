package job

import (
	"context"

	"github.com/equinor/radix-job-scheduler/models"
	log "github.com/sirupsen/logrus"
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
	namespace := jh.kubeUtil.CurrentNamespace()
	log.Debugf("Get Jobs for namespace: %s", namespace)
	kubeJobs, err := kubeClient.BatchV1().Jobs(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	jobs := make([]models.Job, len(kubeJobs.Items))

	for idx, k8sJob := range kubeJobs.Items {
		jobs[idx] = *models.GetJobFromK8sJob(&k8sJob)
	}
	log.Debugf("Found %v jobs for namespace %s", len(jobs), namespace)
	return jobs, nil
}

func (jh *jobHandler) GetJob(ctx context.Context) {
	namespace := "test"
	jobName := "dummy-job-name"
	log.Debugf("Get Job %s for namespace: %s", jobName, namespace)
	log.Debugf("Not found (not implemented) job %s for namespace %s", jobName, namespace)
}

func (jh *jobHandler) CreateJob(ctx context.Context) {
	namespace := "test"
	jobName := "dummy-job-name"
	log.Debugf("Create Job %s for namespace: %s", jobName, namespace)
	log.Debugf("Not created (not implemented) job %s for namespace %s", jobName, namespace)
}

func (jh *jobHandler) DeleteJob(ctx context.Context) {
	namespace := "test"
	jobName := "dummy-job-name"
	log.Debugf("Delete Job %s for namespace: %s", jobName, namespace)
	log.Debugf("Not deleted (not implemented) job %s for namespace %s", jobName, namespace)
}
