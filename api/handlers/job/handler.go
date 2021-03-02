package job

import (
	"context"
	"github.com/pkg/errors"

	"github.com/equinor/radix-job-scheduler/models"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Handler interface {
	GetJobs(ctx context.Context) (*[]models.Job, error)
	GetJob(ctx context.Context, name string) (*models.Job, error)
	CreateJob(ctx context.Context) (*models.Job, error)
	DeleteJob(ctx context.Context) error
}

type jobHandler struct {
	kubeUtil models.KubeUtil
}

func New(kubeUtil models.KubeUtil) Handler {
	return &jobHandler{
		kubeUtil: kubeUtil,
	}
}

func (jh *jobHandler) GetJobs(ctx context.Context) (*[]models.Job, error) {
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
	return &jobs, nil
}

func (jh *jobHandler) GetJob(ctx context.Context, jobName string) (*models.Job, error) {
	kubeClient := jh.kubeUtil.Client()
	namespace := jh.kubeUtil.CurrentNamespace()
	log.Debugf("Get Jobs for namespace: %s", namespace)
	k8job, err := kubeClient.BatchV1().Jobs(namespace).Get(ctx, jobName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if k8job == nil {
		return nil, errors.Errorf("not found Job %s for namespace: %s", jobName, namespace)
	}
	log.Debugf("found Job %s for namespace: %s", jobName, namespace)
	job := models.GetJobFromK8sJob(k8job)
	return job, nil
}

func (jh *jobHandler) CreateJob(ctx context.Context) (*models.Job, error) {
	namespace := "test"
	jobName := "dummy-job-name"
	log.Debugf("Create Job %s for namespace: %s", jobName, namespace)
	log.Debugf("Not created (not implemented) job %s for namespace %s", jobName, namespace)
	return nil, nil
}

func (jh *jobHandler) DeleteJob(ctx context.Context) error {
	namespace := "test"
	jobName := "dummy-job-name"
	log.Debugf("Delete Job %s for namespace: %s", jobName, namespace)
	log.Debugf("Not deleted (not implemented) job %s for namespace %s", jobName, namespace)
	return nil
}
