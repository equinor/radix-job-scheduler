package batchesv1

import (
	"context"
	"sort"

	"github.com/equinor/radix-job-scheduler/api/v1"
	"github.com/equinor/radix-job-scheduler/api/v1/jobs"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type batchHandler struct {
	common *v1.Handler
}

type BatchHandler interface {
	//GetBatches Get status of all batches
	GetBatches() ([]modelsv1.BatchStatus, error)
	//GetBatch Get status of a batch
	GetBatch(batchName string) (*modelsv1.BatchStatus, error)
	//CreateBatch Create a batch with parameters
	CreateBatch(batchScheduleDescription *models.BatchScheduleDescription) (*modelsv1.BatchStatus, error)
	//MaintainHistoryLimit Delete outdated batches
	MaintainHistoryLimit() error
	//DeleteBatch Delete a batch
	DeleteBatch(batchName string) error
}

type completedBatchVersioned struct {
	batchNameV1    string
	batchNameV2    string
	completionTime *metav1.Time
}

//New Constructor of the batch handler
func New(env *models.Env, kube *kube.Kube, kubeClient kubernetes.Interface, radixClient radixclient.Interface) BatchHandler {
	return &batchHandler{
		common: &v1.Handler{
			Kube:         kube,
			KubeClient:   kubeClient,
			RadixClient:  radixClient,
			Env:          env,
			HandlerApiV2: apiv2.New(env, kube, kubeClient, radixClient),
		},
	}
}

//GetBatches Get status of all batches
func (handler *batchHandler) GetBatches() ([]modelsv1.BatchStatus, error) {
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
	var allRadixBatchStatuses []modelsv1.BatchStatus
	for idx, batch := range allBatches {
		allRadixBatchStatuses[idx] = modelsv1.BatchStatus{
			JobStatus: *jobs.GetJobStatusFromJob(handler.common.KubeClient, batch,
				allBatchesPodsMap[batch.Name]),
		}
	}
	log.Debugf("Found %v batches for namespace %s", len(allRadixBatchStatuses), handler.common.Env.RadixDeploymentNamespace)
	radixBatches, err := handler.common.HandlerApiV2.GetRadixBatches()
	if err != nil {
		return nil, err
	}
	for _, radixBatch := range radixBatches {
		radixBatch := radixBatch
		radixBatchStatus := GetBatchStatusFromRadixBatchStatusModel(&radixBatch)
		allRadixBatchStatuses = append(allRadixBatchStatuses, *radixBatchStatus)
	}
	return allRadixBatchStatuses, nil
}

//GetBatch Get status of a batch
func (handler *batchHandler) GetBatch(batchName string) (*modelsv1.BatchStatus, error) {
	log.Debugf("get batches for namespace: %s", handler.common.Env.RadixDeploymentNamespace)
	batch, err := handler.common.GetBatch(batchName)
	if err != nil {
		if errors.IsNotFound(err) {
			radixBatchStatus, err := handler.common.HandlerApiV2.GetRadixBatchStatus(batchName)
			if err != nil {
				return nil, err
			}
			return GetBatchStatusFromRadixBatchStatus(batchName, radixBatchStatus), nil
		}
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
	batchStatus := modelsv1.BatchStatus{
		JobStatus:   *jobs.GetJobStatusFromJob(handler.common.KubeClient, batch, batchPods),
		JobStatuses: make([]modelsv1.JobStatus, len(batchJobs)),
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
func (handler *batchHandler) CreateBatch(batchScheduleDescription *models.BatchScheduleDescription) (*modelsv1.BatchStatus, error) {
	radixBatch, err := handler.common.HandlerApiV2.CreateRadixBatch(batchScheduleDescription)
	if err != nil {
		return nil, err
	}
	return GetBatchStatusFromRadixBatch(radixBatch), nil
}

//DeleteBatch Delete a batch
func (handler *batchHandler) DeleteBatch(batchName string) error {
	log.Debugf("delete batch %s for namespace: %s", batchName, handler.common.Env.RadixDeploymentNamespace)
	fg := metav1.DeletePropagationBackground
	err := handler.common.KubeClient.BatchV1().Jobs(handler.common.Env.RadixDeploymentNamespace).Delete(context.Background(), batchName, metav1.DeleteOptions{PropagationPolicy: &fg})
	if err != nil {
		if errors.IsNotFound(err) {
			return handler.common.HandlerApiV2.DeleteRadixBatch(batchName)
		}
		return err
	}
	return nil
}

//MaintainHistoryLimit Delete outdated batches
func (handler *batchHandler) MaintainHistoryLimit() error {
	completedRadixBatches, err := handler.common.HandlerApiV2.GetCompletedRadixBatchesSortedByCompletionTimeAsc()
	if err != nil {
		return err
	}
	batchList, err := handler.getAllBatches()
	if err != nil {
		return err
	}

	completedBatches := convertRadixBatchesToCompletedBatchVersioned(completedRadixBatches.SucceededRadixBatches)
	log.Debug("maintain history limit for succeeded batches")
	succeededBatches := batchList.Where(func(j *batchv1.Job) bool { return j.Status.Succeeded > 0 })
	completedBatches = append(completedBatches, convertBatchJobsToCompletedBatchVersioned(succeededBatches)...)
	if err = handler.maintainHistoryLimitForBatches(completedBatches,
		handler.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	completedBatches = convertRadixBatchesToCompletedBatchVersioned(completedRadixBatches.NotSucceededRadixBatches)
	log.Debug("maintain history limit for failed batches")
	failedBatches := batchList.Where(func(j *batchv1.Job) bool { return j.Status.Failed > 0 })
	completedBatches = append(completedBatches, convertBatchJobsToCompletedBatchVersioned(failedBatches)...)
	if err = handler.maintainHistoryLimitForBatches(completedBatches,
		handler.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}
	return nil
}

func convertRadixBatchesToCompletedBatchVersioned(radixBatches []*radixv1.RadixBatch) []completedBatchVersioned {
	var completedBatches []completedBatchVersioned
	for _, radixBatch := range radixBatches {
		completedBatches = append(completedBatches, completedBatchVersioned{
			batchNameV2:    radixBatch.GetName(),
			completionTime: radixBatch.Status.Condition.CompletionTime,
		})
	}
	return completedBatches
}

func convertBatchJobsToCompletedBatchVersioned(batchJobs modelsv1.JobList) []completedBatchVersioned {
	var completedBatches []completedBatchVersioned
	for _, radixBatch := range batchJobs {
		completedBatches = append(completedBatches, completedBatchVersioned{
			batchNameV1:    radixBatch.GetName(),
			completionTime: radixBatch.Status.CompletionTime,
		})
	}
	return completedBatches
}

func (handler *batchHandler) maintainHistoryLimitForBatches(completedBatchesVersioned []completedBatchVersioned, historyLimit int) error {
	numToDelete := len(completedBatchesVersioned) - historyLimit
	if numToDelete <= 0 {
		log.Debug("no history batches to delete")
		return nil
	}
	log.Debugf("history batches to delete: %v", numToDelete)

	sortedCompletedBatchesVersioned := sortCompletedBatchesByCompletionTimeAsc(completedBatchesVersioned)
	for i := 0; i < numToDelete; i++ {
		batchVersioned := sortedCompletedBatchesVersioned[i]
		if len(batchVersioned.batchNameV1) > 0 {
			log.Debugf("deleting batch batch job %s", batchVersioned.batchNameV1)
			if err := handler.DeleteBatch(batchVersioned.batchNameV1); err != nil {
				return err
			}
			continue
		}
		log.Debugf("deleting batch %s", batchVersioned.batchNameV2)
		if err := handler.common.HandlerApiV2.DeleteRadixBatch(batchVersioned.batchNameV2); err != nil {
			return err
		}
	}
	return nil
}

func sortCompletedBatchesByCompletionTimeAsc(completedBatchesVersioned []completedBatchVersioned) []completedBatchVersioned {
	sort.Slice(completedBatchesVersioned, func(i, j int) bool {
		batch1 := (completedBatchesVersioned)[i]
		batch2 := (completedBatchesVersioned)[j]
		return isCompletedBatch1CompletedBefore2(batch1, batch2)
	})
	return completedBatchesVersioned
}

func isCompletedBatch1CompletedBefore2(batchVersioned1 completedBatchVersioned, batchVersioned2 completedBatchVersioned) bool {
	batch1CompletedTime := getCompletionTimeFrom(batchVersioned1)
	batch2CompletedTime := getCompletionTimeFrom(batchVersioned2)

	return batch1CompletedTime.Before(batch2CompletedTime)
}

func getCompletionTimeFrom(batchVersioned completedBatchVersioned) *metav1.Time {
	if batchVersioned.completionTime.IsZero() {
		return batchVersioned.completionTime
	}
	return batchVersioned.completionTime
}

func (handler *batchHandler) getAllBatches() (modelsv1.JobList, error) {
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

func (handler *batchHandler) getBatchJobs(batchName string) (modelsv1.JobList, error) {
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
