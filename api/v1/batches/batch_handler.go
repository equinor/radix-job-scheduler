package batchesv1

import (
	"context"
	"fmt"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/api/v1"
	"github.com/equinor/radix-job-scheduler/api/v1/jobs"
	v12 "github.com/equinor/radix-job-scheduler/models/v1"
	"sort"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type batchHandler struct {
	common *v1.Handler
}

type BatchHandler interface {
	//GetBatches Get status of all batches
	GetBatches() ([]v12.BatchStatus, error)
	//GetBatch Get status of a batch
	GetBatch(batchName string) (*v12.BatchStatus, error)
	//CreateBatch Create a batch with parameters
	CreateBatch(batchScheduleDescription *models.BatchScheduleDescription) (*v12.BatchStatus, error)
	//MaintainHistoryLimit Delete outdated batches
	MaintainHistoryLimit() error
	//DeleteBatch Delete a batch
	DeleteBatch(batchName string) error
}

//New Constructor of the batch handler
func New(env *models.Env, kube *kube.Kube, kubeClient kubernetes.Interface, radixClient radixclient.Interface) BatchHandler {
	return &batchHandler{
		common: &v1.Handler{
			Kube:        kube,
			KubeClient:  kubeClient,
			RadixClient: radixClient,
			Env:         env,
		},
	}
}

//GetBatches Get status of all batches
func (handler *batchHandler) GetBatches() ([]v12.BatchStatus, error) {
	log.Debugf("Get batches for the namespace: %s", handler.common.Env.RadixDeploymentNamespace)

	allBatches, err := handler.getAllBatches()
	if err != nil {
		return nil, err
	}

	allBatchesPods, err := handler.common.GetPodsForLabelSelector(getLabelSelectorForAllBatchesPods())
	if err != nil {
		return nil, err
	}
	allBatchesPodsMap := v1.GetPodsToJobNameMap(allBatchesPods)
	allBatchStatuses := make([]v12.BatchStatus, len(allBatches))
	for idx, batch := range allBatches {
		allBatchStatuses[idx] = v12.BatchStatus{
			JobStatus: *jobs.GetJobStatusFromJob(handler.common.KubeClient, batch,
				allBatchesPodsMap[batch.Name]),
		}
	}

	log.Debugf("Found %v batches for namespace %s", len(allBatchStatuses), handler.common.Env.RadixDeploymentNamespace)
	return allBatchStatuses, nil
}

//GetBatch Get status of a batch
func (handler *batchHandler) GetBatch(batchName string) (*v12.BatchStatus, error) {
	log.Debugf("get batches for namespace: %s", handler.common.Env.RadixDeploymentNamespace)
	batch, err := handler.common.GetBatch(batchName)
	if err != nil {
		return nil, err
	}
	log.Debugf("found Batch %s for namespace: %s", batchName, handler.common.Env.RadixDeploymentNamespace)
	batchJobs, err := handler.getBatchJobs(batchName)
	if err != nil {
		return nil, err
	}
	batchPods, err := handler.common.GetPodsForLabelSelector(getLabelSelectorForBatchPods(batchName))
	if err != nil {
		return nil, err
	}
	batchStatus := v12.BatchStatus{
		JobStatus:   *jobs.GetJobStatusFromJob(handler.common.KubeClient, batch, batchPods),
		JobStatuses: make([]v12.JobStatus, len(batchJobs)),
	}

	batchJobsPods, err := handler.common.GetPodsForLabelSelector(getLabelSelectorForBatchObjects(batchName))
	if err != nil {
		return nil, err
	}
	batchJobsPodsMap := v1.GetPodsToJobNameMap(batchJobsPods)
	for idx, batchJob := range batchJobs {
		batchStatus.JobStatuses[idx] = *jobs.GetJobStatusFromJob(handler.common.KubeClient, batchJob, batchJobsPodsMap[batchJob.Name])
	}

	log.Debugf("Found %v jobs for the batch '%s' for namespace '%s'", len(batchJobs), batchName,
		handler.common.Env.RadixDeploymentNamespace)
	return &batchStatus, nil
}

//CreateBatch Create a batch with parameters
func (handler *batchHandler) CreateBatch(batchScheduleDescription *models.BatchScheduleDescription) (*v12.BatchStatus, error) {
	namespace := handler.common.Env.RadixDeploymentNamespace
	radixComponentName := handler.common.Env.RadixComponentName
	radixDeploymentName := handler.common.Env.RadixDeploymentName
	log.Debugf("create batch for namespace: %s", namespace)

	radixDeployment, err := handler.common.RadixClient.RadixV1().RadixDeployments(namespace).
		Get(context.Background(), radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, apiErrors.NewNotFound("radix deployment", radixDeploymentName)
	}

	radixJobComponent := radixDeployment.GetJobComponentByName(radixComponentName)
	if radixJobComponent == nil {
		return nil, apiErrors.NewNotFound("job component", radixComponentName)
	}

	batchName := generateBatchName(radixComponentName)
	appName := radixDeployment.Spec.AppName

	createdBatch, err := handler.createBatch(namespace, appName, batchName, radixJobComponent, batchScheduleDescription)
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	log.Debug(fmt.Sprintf("created batch %s for component %s, environment %s, in namespace: %s", createdBatch.Name,
		radixComponentName, radixDeployment.Spec.Environment, namespace))
	return GetBatchStatusFromJob(handler.common.KubeClient, createdBatch, nil)
}

//DeleteBatch Delete a batch
func (handler *batchHandler) DeleteBatch(batchName string) error {
	log.Debugf("delete batch %s for namespace: %s", batchName, handler.common.Env.RadixDeploymentNamespace)
	fg := metav1.DeletePropagationBackground
	return handler.common.KubeClient.BatchV1().Jobs(handler.common.Env.RadixDeploymentNamespace).Delete(context.Background(), batchName, metav1.DeleteOptions{PropagationPolicy: &fg})
}

//MaintainHistoryLimit Delete outdated batches
func (handler *batchHandler) MaintainHistoryLimit() error {
	batchList, err := handler.getAllBatches()
	if err != nil {
		return err
	}

	log.Debug("maintain history limit for succeeded batches")
	succeededBatches := batchList.Where(func(j *batchv1.Job) bool { return j.Status.Succeeded > 0 })
	if err = handler.maintainHistoryLimitForBatches(succeededBatches,
		handler.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	log.Debug("maintain history limit for failed batches")
	failedBatches := batchList.Where(func(j *batchv1.Job) bool { return j.Status.Failed > 0 })
	if err = handler.maintainHistoryLimitForBatches(failedBatches,
		handler.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	return nil
}

func (handler *batchHandler) maintainHistoryLimitForBatches(batches []*batchv1.Job, historyLimit int) error {
	numToDelete := len(batches) - historyLimit
	if numToDelete <= 0 {
		log.Debug("no history batches to delete")
		return nil
	}
	log.Debugf("history batches to delete: %v", numToDelete)

	sortedBatches := sortRJSchByCompletionTimeAsc(batches)
	for i := 0; i < numToDelete; i++ {
		batch := sortedBatches[i]
		log.Debugf("deleting batch %s", batch.Name)
		if err := handler.DeleteBatch(batch.Name); err != nil {
			return err
		}
	}
	return nil
}

func sortRJSchByCompletionTimeAsc(batches []*batchv1.Job) []*batchv1.Job {
	sort.Slice(batches, func(i, j int) bool {
		batch1 := (batches)[i]
		batch2 := (batches)[j]
		return isRJS1CompletedBeforeRJS2(batch1, batch2)
	})
	return batches
}

func isRJS1CompletedBeforeRJS2(batch1 *batchv1.Job, batch2 *batchv1.Job) bool {
	rd1ActiveFrom := getCompletionTimeFrom(batch1)
	rd2ActiveFrom := getCompletionTimeFrom(batch2)

	return rd1ActiveFrom.Before(rd2ActiveFrom)
}

func getCompletionTimeFrom(batch *batchv1.Job) *metav1.Time {
	if batch.Status.CompletionTime.IsZero() {
		return &batch.CreationTimestamp
	}
	return batch.Status.CompletionTime
}

func generateBatchName(jobComponentName string) string {
	timestamp := time.Now().Format("20060102150405")
	jobTag := strings.ToLower(commonUtils.RandString(8))
	return fmt.Sprintf("batch-%s-%s-%s", jobComponentName, timestamp, jobTag)
}

func (handler *batchHandler) createBatch(namespace, appName, batchName string, radixJobComponent *radixv1.RadixDeployJobComponent, batchScheduleDescription *models.BatchScheduleDescription) (*batchv1.Job, error) {
	//TODO
	return nil, nil
}

func (handler *batchHandler) getAllBatches() (v12.JobList, error) {
	kubeBatches, err := handler.common.KubeClient.
		BatchV1().
		Jobs(handler.common.Env.RadixDeploymentNamespace).
		List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: v1.GetLabelSelectorForBatches(handler.common.Env.RadixComponentName),
			},
		)

	if err != nil {
		return nil, err
	}

	return slice.PointersOf(kubeBatches.Items).([]*batchv1.Job), nil
}

func getLabelSelectorForBatchPods(batchName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixJobTypeLabel:   kube.RadixJobTypeBatchSchedule,
		kube.RadixBatchNameLabel: batchName,
	}).String()
}

func getLabelSelectorForAllBatchesPods() string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixJobTypeLabel: kube.RadixJobTypeBatchSchedule,
	}).String()
}

func getLabelSelectorForBatchObjects(batchName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
		kube.RadixBatchNameLabel: batchName,
	}).String()
}

func (handler *batchHandler) getBatchJobs(batchName string) (v12.JobList, error) {
	kubeJobs, err := handler.common.KubeClient.
		BatchV1().
		Jobs(handler.common.Env.RadixDeploymentNamespace).
		List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: getLabelSelectorForBatchObjects(batchName),
			},
		)

	if err != nil {
		return nil, err
	}

	return slice.PointersOf(kubeJobs.Items).([]*batchv1.Job), nil
}
