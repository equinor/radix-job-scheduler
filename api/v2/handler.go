package apiv2

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	commonErrors "github.com/equinor/radix-common/utils/errors"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/imdario/mergo"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	//Max size of the secret description, including description, metadata, base64 encodes secret values, etc.
	maxPayloadSecretSize = 1024 * 512 //0.5MB
	//Standard secret description, metadata, etc.
	payloadSecretAuxDataSize = 600
	//Each entry in a secret Data has name, etc.
	payloadSecretEntryAuxDataSize = 128
)

var (
	authTransformer mergo.Transformers = mergoutils.CombinedTransformer{Transformers: []mergo.Transformers{mergoutils.BoolPtrTransformer{}}}
	defaultSrc                         = rand.NewSource(time.Now().UnixNano())
)

type handler struct {
	kubeUtil    *kube.Kube
	env         *models.Env
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
}

type Handler interface {
	//GetRadixBatches Get status of all batches
	GetRadixBatches() ([]modelsv2.RadixBatch, error)
	//GetRadixBatchSingleJobs Get status of all single jobs
	GetRadixBatchSingleJobs() ([]modelsv2.RadixBatch, error)
	//GetRadixBatch Get a batch
	GetRadixBatch(string) (*modelsv2.RadixBatch, error)
	//CreateRadixBatch Create a batch with parameters
	CreateRadixBatch(*common.BatchScheduleDescription) (*modelsv2.RadixBatch, error)
	//CreateRadixBatchSingleJob Create a batch with single job parameters
	CreateRadixBatchSingleJob(*common.JobScheduleDescription) (*modelsv2.RadixBatch, error)
	//MaintainHistoryLimit Delete outdated batches
	MaintainHistoryLimit() error
	// GarbageCollectPayloadSecrets Delete orphaned payload secrets
	GarbageCollectPayloadSecrets() error
	//DeleteRadixBatch Delete a batch
	DeleteRadixBatch(string) error
	// GetCompletedRadixBatchesSortedByCompletionTimeAsc Gets completed RadixBatch lists for env.RadixComponentName
	GetCompletedRadixBatchesSortedByCompletionTimeAsc() (*CompletedRadixBatches, error)
	//StopRadixBatch Stop a batch
	StopRadixBatch(string) error
	//StopRadixBatchJob Stop a batch job
	StopRadixBatchJob(string, string) error
}

// CompletedRadixBatches Completed RadixBatch lists
type CompletedRadixBatches struct {
	SucceededRadixBatches    []*modelsv2.RadixBatch
	NotSucceededRadixBatches []*modelsv2.RadixBatch
	SucceededSingleJobs      []*modelsv2.RadixBatch
	NotSucceededSingleJobs   []*modelsv2.RadixBatch
}

// New Constructor of the batch handler
func New(env *models.Env, kube *kube.Kube, kubeClient kubernetes.Interface, radixClient radixclient.Interface) Handler {
	return &handler{
		kubeUtil:    kube,
		kubeClient:  kubeClient,
		radixClient: radixClient,
		env:         env,
	}
}

type radixBatchJobWithDescription struct {
	radixBatchJob          *radixv1.RadixBatchJob
	jobScheduleDescription *common.JobScheduleDescription
}

// GetRadixBatches Get statuses of all batches
func (h *handler) GetRadixBatches() ([]modelsv2.RadixBatch, error) {
	log.Debugf("Get batches for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatus(kube.RadixBatchTypeBatch)
}

// GetRadixBatchSingleJobs Get statuses of all single jobs
func (h *handler) GetRadixBatchSingleJobs() ([]modelsv2.RadixBatch, error) {
	log.Debugf("Get sigle jobs for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatus(kube.RadixBatchTypeJob)
}

func (h *handler) getRadixBatchStatus(radixBatchType kube.RadixBatchType) ([]modelsv2.RadixBatch, error) {
	radixBatches, err := h.getRadixBatches(
		radixLabels.ForComponentName(h.env.RadixComponentName),
		radixLabels.ForBatchType(radixBatchType),
	)
	if err != nil {
		return nil, err
	}
	log.Debugf("Found %v batches for namespace %s", len(radixBatches), h.env.RadixDeploymentNamespace)

	var radixBatchStatuses []modelsv2.RadixBatch
	for _, radixBatch := range radixBatches {
		radixBatchStatuses = append(radixBatchStatuses, convertToRadixBatch(radixBatch))
	}
	return radixBatchStatuses, nil
}

func convertToRadixBatch(radixBatch *radixv1.RadixBatch) modelsv2.RadixBatch {
	batch := modelsv2.RadixBatch{
		Name:         radixBatch.GetName(),
		BatchType:    radixBatch.Labels[kube.RadixBatchTypeLabel],
		CreationTime: utils.FormatTime(pointers.Ptr(radixBatch.GetCreationTimestamp())),
		Started:      utils.FormatTime(radixBatch.Status.Condition.ActiveTime),
		Ended:        utils.FormatTime(radixBatch.Status.Condition.CompletionTime),
		Status:       GetRadixBatchStatus(radixBatch).String(),
		Message:      radixBatch.Status.Condition.Message,
		JobStatuses:  getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatch.Status.JobStatuses),
	}
	return batch
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []modelsv2.RadixBatchJobStatus {
	radixBatchJobsStatuses := getRadixBatchJobsStatusesMap(radixBatchJobStatuses)
	var jobStatuses []modelsv2.RadixBatchJobStatus
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		radixBatchJobStatus := modelsv2.RadixBatchJobStatus{
			Name:  radixBatchJob.Name,
			JobId: radixBatchJob.JobId,
		}
		if jobStatus, ok := radixBatchJobsStatuses[radixBatchJob.Name]; ok {
			radixBatchJobStatus.CreationTime = utils.FormatTime(jobStatus.CreationTime)
			radixBatchJobStatus.Started = utils.FormatTime(jobStatus.StartTime)
			radixBatchJobStatus.Ended = utils.FormatTime(jobStatus.EndTime)
			radixBatchJobStatus.Status = GetRadixBatchJobStatusFromPhase(radixBatchJob, jobStatus.Phase).String()
			radixBatchJobStatus.Message = jobStatus.Message
		}
		jobStatuses = append(jobStatuses, radixBatchJobStatus)
	}
	return jobStatuses
}

func getRadixBatchJobsStatusesMap(radixBatchJobStatuses []radixv1.RadixBatchJobStatus) map[string]radixv1.RadixBatchJobStatus {
	radixBatchJobsStatuses := make(map[string]radixv1.RadixBatchJobStatus)
	for _, jobStatus := range radixBatchJobStatuses {
		jobStatus := jobStatus
		radixBatchJobsStatuses[jobStatus.Name] = jobStatus
	}
	return radixBatchJobsStatuses
}

// GetRadixBatchStatus Gets the batch status
func GetRadixBatchStatus(radixBatch *radixv1.RadixBatch) (status common.ProgressStatus) {
	status = common.Waiting
	switch {
	case radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeActive:
		status = common.Running
	case radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted:
		status = common.Succeeded
		if slice.Any(radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Phase == radixv1.BatchJobPhaseFailed
		}) {
			status = common.Failed
		}
	}
	return
}

// GetRadixBatchJobStatusFromPhase Get batch job status by job phase
func GetRadixBatchJobStatusFromPhase(job radixv1.RadixBatchJob, phase radixv1.RadixBatchJobPhase) (status common.ProgressStatus) {
	status = common.Waiting

	switch phase {
	case radixv1.BatchJobPhaseActive:
		status = common.Running
	case radixv1.BatchJobPhaseSucceeded:
		status = common.Succeeded
	case radixv1.BatchJobPhaseFailed:
		status = common.Failed
	case radixv1.BatchJobPhaseStopped:
		status = common.Stopped
	}

	var stop bool
	if job.Stop != nil {
		stop = *job.Stop
	}

	if stop && (status == common.Waiting || status == common.Running) {
		status = common.Stopping
	}

	return
}

// GetRadixBatch Get status of a batch
func (h *handler) GetRadixBatch(batchName string) (*modelsv2.RadixBatch, error) {
	log.Debugf("get batch status for the batch %s for namespace: %s", batchName, h.env.RadixDeploymentNamespace)
	radixBatch, err := h.getRadixBatch(batchName)
	if err != nil {
		return nil, err
	}
	return pointers.Ptr(convertToRadixBatch(radixBatch)), nil
}

// CreateRadixBatch Create a batch with parameters
func (h *handler) CreateRadixBatch(batchScheduleDescription *common.BatchScheduleDescription) (*modelsv2.RadixBatch, error) {
	return h.createRadixBatchOrJob(batchScheduleDescription, kube.RadixBatchTypeBatch)
}

func (h *handler) createRadixBatchOrJob(batchScheduleDescription *common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*modelsv2.RadixBatch, error) {
	namespace := h.env.RadixDeploymentNamespace
	radixComponentName := h.env.RadixComponentName
	radixDeploymentName := h.env.RadixDeploymentName
	log.Debugf("create batch for namespace: %s", namespace)

	radixDeployment, err := h.radixClient.RadixV1().RadixDeployments(namespace).
		Get(context.Background(), radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, apiErrors.NewNotFound("radix deployment", radixDeploymentName)
	}

	radixJobComponent := radixDeployment.GetJobComponentByName(radixComponentName)
	if radixJobComponent == nil {
		return nil, apiErrors.NewNotFound("job component", radixComponentName)
	}

	appName := radixDeployment.Spec.AppName

	createdRadixBatch, err := h.createBatch(namespace, appName, radixDeployment.GetName(), radixJobComponent, batchScheduleDescription, radixBatchType)
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	log.Debug(fmt.Sprintf("created batch %s for component %s, environment %s, in namespace: %s", createdRadixBatch.GetName(),
		radixComponentName, radixDeployment.Spec.Environment, namespace))
	return pointers.Ptr(convertToRadixBatch(createdRadixBatch)), nil
}

// CreateRadixBatchSingleJob Create a batch single job with parameters
func (h *handler) CreateRadixBatchSingleJob(jobScheduleDescription *common.JobScheduleDescription) (*modelsv2.RadixBatch, error) {
	if jobScheduleDescription == nil {
		return nil, fmt.Errorf("missing expected job description")
	}
	return h.createRadixBatchOrJob(&common.BatchScheduleDescription{
		JobScheduleDescriptions:        []common.JobScheduleDescription{*jobScheduleDescription},
		DefaultRadixJobComponentConfig: nil,
	}, kube.RadixBatchTypeJob)
}

// DeleteRadixBatch Delete a batch
func (h *handler) DeleteRadixBatch(batchName string) error {
	log.Debugf("delete batch %s for namespace: %s", batchName, h.env.RadixDeploymentNamespace)
	fg := metav1.DeletePropagationBackground
	err := h.radixClient.RadixV1().RadixBatches(h.env.RadixDeploymentNamespace).Delete(context.Background(), batchName, metav1.DeleteOptions{PropagationPolicy: &fg})
	if err != nil {
		return err
	}
	secrets, err := h.GetSecretsForRadixBatch(batchName)
	if err != nil {
		return err
	}
	var errs []error
	for _, secret := range secrets {
		err := h.DeleteSecret(secret)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return commonErrors.Concat(errs)
}

// StopRadixBatch Stop a batch
func (h *handler) StopRadixBatch(batchName string) error {
	namespace := h.env.RadixDeploymentNamespace
	log.Debugf("stop batch %s for namespace: %s", batchName, namespace)
	radixBatch, err := h.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted {
		return fmt.Errorf("cannot stop completed batch %s", batchName)
	}

	newRadixBatch := radixBatch.DeepCopy()
	radixBatchJobsStatusMap := getRadixBatchJobsStatusesMap(newRadixBatch.Status.JobStatuses)
	for jobIndex, radixBatchJob := range newRadixBatch.Spec.Jobs {
		if jobStatus, ok := radixBatchJobsStatusMap[radixBatchJob.Name]; ok &&
			(isRadixBatchJobSucceeded(jobStatus) || isRadixBatchJobFailed(jobStatus)) {
			continue
		}
		newRadixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
	}
	return h.updateRadixBatch(newRadixBatch)
}

// StopRadixBatchJob Stop a batch job
func (h *handler) StopRadixBatchJob(batchName, jobName string) error {
	namespace := h.env.RadixDeploymentNamespace
	log.Debugf("stop a job %s in the batch %s for namespace: %s", jobName, batchName, namespace)
	radixBatch, err := h.radixClient.RadixV1().RadixBatches(namespace).Get(context.Background(), batchName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted {
		return fmt.Errorf("cannot stop the job %s in the completed batch %s", jobName, batchName)
	}

	newRadixBatch := radixBatch.DeepCopy()
	radixBatchJobsStatusMap := getRadixBatchJobsStatusesMap(newRadixBatch.Status.JobStatuses)
	for jobIndex, radixBatchJob := range newRadixBatch.Spec.Jobs {
		if !strings.EqualFold(radixBatchJob.Name, jobName) {
			continue
		}
		if jobStatus, ok := radixBatchJobsStatusMap[radixBatchJob.Name]; ok &&
			(isRadixBatchJobSucceeded(jobStatus) || isRadixBatchJobFailed(jobStatus)) {
			return fmt.Errorf("cannot stop the job %s with the status %s in the batch %s", jobName, string(jobStatus.Phase), batchName)
		}
		newRadixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
		return h.updateRadixBatch(newRadixBatch)
	}
	return fmt.Errorf("not found a job %s in the batch %s", jobName, batchName)
}

func (h *handler) updateRadixBatch(radixBatch *radixv1.RadixBatch) error {
	namespace := h.env.RadixDeploymentNamespace
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := h.radixClient.RadixV1().RadixBatches(namespace).Update(context.Background(), radixBatch, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to patch RadixBatch object: %v", err)
	}
	log.Debugf("Patched RadixBatch: %s in namespace %s", radixBatch.GetName(), namespace)
	return nil
}

// MaintainHistoryLimit Delete outdated batches
func (h *handler) MaintainHistoryLimit() error {
	completedRadixBatches, _ := h.GetCompletedRadixBatchesSortedByCompletionTimeAsc()

	historyLimit := h.env.RadixJobSchedulersPerEnvironmentHistoryLimit
	log.Debug("maintain history limit for succeeded batches")
	var errs []error
	if err := h.maintainHistoryLimitForBatches(completedRadixBatches.SucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	log.Debug("maintain history limit for not succeeded batches")
	if err := h.maintainHistoryLimitForBatches(completedRadixBatches.NotSucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	log.Debug("maintain history limit for succeeded single jobs")
	if err := h.maintainHistoryLimitForBatches(completedRadixBatches.SucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	log.Debug("maintain history limit for not succeeded single jobs")
	if err := h.maintainHistoryLimitForBatches(completedRadixBatches.NotSucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	log.Debug("delete orphaned payload secrets")
	err := h.GarbageCollectPayloadSecrets()
	if err != nil {
		errs = append(errs, err)
	}
	return commonErrors.Concat(errs)
}

// GetCompletedRadixBatchesSortedByCompletionTimeAsc Gets completed RadixBatch lists for env.RadixComponentName
func (h *handler) GetCompletedRadixBatchesSortedByCompletionTimeAsc() (*CompletedRadixBatches, error) {
	radixBatches, err := h.getRadixBatches(radixLabels.ForComponentName(h.env.RadixComponentName))
	if err != nil {
		return nil, err
	}
	radixBatches = sortRJSchByCompletionTimeAsc(radixBatches)
	return &CompletedRadixBatches{
		SucceededRadixBatches:    getSucceededRadixBatches(radixBatches),
		NotSucceededRadixBatches: getNotSucceededRadixBatches(radixBatches),
		SucceededSingleJobs:      getSucceededSingleJobs(radixBatches),
		NotSucceededSingleJobs:   getNotSucceededSingleJobs(radixBatches),
	}, nil
}

func getNotSucceededRadixBatches(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	return convertToRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchNotSucceeded(radixBatch)
	}))
}

func getSucceededRadixBatches(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	radixBatches = slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchSucceeded(radixBatch)
	})
	return convertToRadixBatches(radixBatches)
}

func getNotSucceededSingleJobs(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	return convertToRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchNotSucceeded(radixBatch)
	}))
}

func getSucceededSingleJobs(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	return convertToRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchSucceeded(radixBatch)
	}))
}

func convertToRadixBatches(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	var batches []*modelsv2.RadixBatch
	for _, radixBatch := range radixBatches {
		batches = append(batches, pointers.Ptr(convertToRadixBatch(radixBatch)))
	}
	return batches
}

func radixBatchHasType(radixBatch *radixv1.RadixBatch, radixBatchType kube.RadixBatchType) bool {
	return radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(radixBatchType)
}

func isRadixBatchSucceeded(batch *radixv1.RadixBatch) bool {
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && slice.All(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return isRadixBatchJobSucceeded(jobStatus)
	})
}

func isRadixBatchNotSucceeded(batch *radixv1.RadixBatch) bool {
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && !slice.Any(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return isRadixBatchJobSucceeded(jobStatus)
	})
}

func isRadixBatchJobSucceeded(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded || jobStatus.Phase == radixv1.BatchJobPhaseStopped
}

func isRadixBatchJobFailed(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseFailed
}

func (h *handler) maintainHistoryLimitForBatches(radixBatchesSortedByCompletionTimeAsc []*modelsv2.RadixBatch, historyLimit int) error {
	numToDelete := len(radixBatchesSortedByCompletionTimeAsc) - historyLimit
	if numToDelete <= 0 {
		log.Debug("no history batches to delete")
		return nil
	}
	log.Debugf("history batches to delete: %v", numToDelete)

	for i := 0; i < numToDelete; i++ {
		batch := radixBatchesSortedByCompletionTimeAsc[i]
		log.Debugf("deleting batch %s", batch.Name)
		if err := h.DeleteRadixBatch(batch.Name); err != nil {
			return err
		}
	}
	return nil
}

func sortRJSchByCompletionTimeAsc(batches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	sort.Slice(batches, func(i, j int) bool {
		batch1 := (batches)[i]
		batch2 := (batches)[j]
		return isRJS1CompletedBeforeRJS2(batch1, batch2)
	})
	return batches
}

func isRJS1CompletedBeforeRJS2(batch1 *radixv1.RadixBatch, batch2 *radixv1.RadixBatch) bool {
	rd1ActiveFrom := getCompletionTimeFrom(batch1)
	rd2ActiveFrom := getCompletionTimeFrom(batch2)

	return rd1ActiveFrom.Before(rd2ActiveFrom)
}

func getCompletionTimeFrom(radixBatch *radixv1.RadixBatch) *metav1.Time {
	if radixBatch.Status.Condition.CompletionTime.IsZero() {
		return pointers.Ptr(radixBatch.GetCreationTimestamp())
	}
	return radixBatch.Status.Condition.CompletionTime
}

func generateBatchName(jobComponentName string) string {
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("batch-%s-%s-%s", getJobComponentNamePart(jobComponentName), timestamp, strings.ToLower(utils.RandString(8)))
}

func getJobComponentNamePart(jobComponentName string) string {
	componentNamePart := jobComponentName
	if len(componentNamePart) > 12 {
		componentNamePart = componentNamePart[:12]
	}
	return fmt.Sprintf("%s%s", componentNamePart, strings.ToLower(utils.RandString(16-len(componentNamePart))))
}

func (h *handler) createBatch(namespace, appName, radixDeploymentName string, radixJobComponent *radixv1.RadixDeployJobComponent, batchScheduleDescription *common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	batchName := generateBatchName(radixJobComponent.GetName())
	radixJobComponentName := radixJobComponent.GetName()
	radixBatchJobs, err := h.buildRadixBatchJobs(namespace, appName, radixJobComponentName, batchName, batchScheduleDescription, radixJobComponent.Payload, radixBatchType)
	if err != nil {
		return nil, err
	}

	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name: batchName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(radixJobComponentName),
				radixLabels.ForBatchType(radixBatchType),
			),
		},
		Spec: radixv1.RadixBatchSpec{
			Jobs: radixBatchJobs,
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  radixJobComponentName,
			},
		},
	}
	return h.radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &radixBatch,
		metav1.CreateOptions{})
}

func (h *handler) buildRadixBatchJobs(namespace, appName, radixJobComponentName, batchName string, batchScheduleDescription *common.BatchScheduleDescription, radixJobComponentPayload *radixv1.RadixJobComponentPayload, radixBatchType kube.RadixBatchType) ([]radixv1.RadixBatchJob, error) {
	var radixBatchJobWithDescriptions []radixBatchJobWithDescription
	var errs []error
	for jobIndex, jobScheduleDescription := range batchScheduleDescription.JobScheduleDescriptions {
		jobName := createJobName(radixBatchType, jobIndex)
		radixBatchJob, err := buildRadixBatchJob(jobName, &jobScheduleDescription, batchScheduleDescription.DefaultRadixJobComponentConfig)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		radixBatchJobWithDescriptions = append(radixBatchJobWithDescriptions, radixBatchJobWithDescription{
			radixBatchJob:          radixBatchJob,
			jobScheduleDescription: &jobScheduleDescription,
		})
	}
	if len(errs) > 0 {
		return nil, commonErrors.Concat(errs)
	}
	radixJobComponentHasPayloadPath := radixJobComponentPayload != nil && len(radixJobComponentPayload.Path) > 0
	err := h.createRadixBatchJobPayloadSecrets(namespace, appName, radixJobComponentName, batchName, radixBatchJobWithDescriptions, radixJobComponentHasPayloadPath)
	if err != nil {
		return nil, err
	}
	var radixBatchJobs []radixv1.RadixBatchJob
	for _, item := range radixBatchJobWithDescriptions {
		radixBatchJobs = append(radixBatchJobs, *item.radixBatchJob)
	}
	return radixBatchJobs, nil
}

func createJobName(radixBatchType kube.RadixBatchType, jobIndex int) string {
	if radixBatchType == kube.RadixBatchTypeBatch {
		return fmt.Sprintf("job%d", jobIndex)
	}
	return strings.ToLower(utils.RandStringSeed(8, defaultSrc))
}

func (h *handler) createRadixBatchJobPayloadSecrets(namespace, appName, radixJobComponentName, batchName string, radixJobWithPayloadEntries []radixBatchJobWithDescription, radixJobComponentHasPayloadPath bool) error {
	accumulatedSecretSize := 0
	var payloadSecrets []*corev1.Secret
	payloadsSecret := buildSecret(appName, radixJobComponentName, batchName, 0)
	payloadSecrets = append(payloadSecrets, payloadsSecret)
	var errs []error
	for jobIndex, radixJobWithDescriptions := range radixJobWithPayloadEntries {
		payload := []byte(strings.TrimSpace(radixJobWithDescriptions.jobScheduleDescription.Payload))
		if len(payload) == 0 {
			log.Debugf("no payload in the job #%d", jobIndex)
			continue
		}
		if !radixJobComponentHasPayloadPath {
			errs = append(errs, fmt.Errorf("missing an expected payload path, but there is a payload in the job #%d", jobIndex))
			continue
		}

		radixBatchJob := radixJobWithDescriptions.radixBatchJob
		payloadBase64 := base64.RawStdEncoding.EncodeToString(payload)
		secretEntrySize := len(payloadBase64) + len(radixBatchJob.Name) + payloadSecretEntryAuxDataSize //preliminary estimate of a payload secret entry
		if payloadSecretAuxDataSize+accumulatedSecretSize+secretEntrySize > maxPayloadSecretSize {
			if len(payloadsSecret.Data) == 0 {
				//this is the first entry in the secret, and it is too large to be stored to the secret - no reason to create new secret.
				return fmt.Errorf("payload is too large in the job #%d - its base64 size is %d bytes, but it is expected to be less then %d bytes", jobIndex, secretEntrySize, maxPayloadSecretSize)
			}
			payloadsSecret = buildSecret(appName, radixJobComponentName, batchName, len(payloadSecrets))
			payloadSecrets = append(payloadSecrets, payloadsSecret)
			accumulatedSecretSize = 0
		}

		payloadsSecret.Data[radixBatchJob.Name] = payload
		accumulatedSecretSize = accumulatedSecretSize + secretEntrySize

		radixBatchJob.PayloadSecretRef = &radixv1.PayloadSecretKeySelector{
			LocalObjectReference: radixv1.LocalObjectReference{Name: payloadsSecret.GetName()},
			Key:                  radixBatchJob.Name,
		}
	}
	if len(errs) > 0 {
		return commonErrors.Concat(errs)
	}
	return h.createSecrets(namespace, payloadSecrets)
}

func (h *handler) createSecrets(namespace string, secrets []*corev1.Secret) error {
	for _, secret := range secrets {
		if secret.Data == nil || len(secret.Data) == 0 {
			continue //if Data is empty, the secret is not used in any jobs
		}
		_, err := h.kubeUtil.KubeClient().CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func buildSecret(appName, radixJobComponentName, batchName string, secretIndex int) *corev1.Secret {
	secretName := fmt.Sprintf("%s-payloads-%d", batchName, secretIndex)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(radixJobComponentName),
				radixLabels.ForBatchName(batchName),
			),
		},
		Data: make(map[string][]byte),
	}
}

func buildRadixBatchJob(jobName string, jobScheduleDescription *common.JobScheduleDescription, defaultJobScheduleDescription *common.RadixJobComponentConfig) (*radixv1.RadixBatchJob, error) {
	err := applyDefaultJobDescriptionProperties(jobScheduleDescription, defaultJobScheduleDescription)
	if err != nil {
		return nil, err
	}
	return &radixv1.RadixBatchJob{
		Name:             jobName,
		JobId:            jobScheduleDescription.JobId,
		Resources:        jobScheduleDescription.Resources,
		Node:             jobScheduleDescription.Node,
		TimeLimitSeconds: jobScheduleDescription.TimeLimitSeconds,
		BackoffLimit:     jobScheduleDescription.BackoffLimit,
	}, nil
}

func (h *handler) getRadixBatches(labels ...map[string]string) ([]*radixv1.RadixBatch, error) {
	radixBatchList, err := h.radixClient.
		RadixV1().
		RadixBatches(h.env.RadixDeploymentNamespace).
		List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: radixLabels.Merge(labels...).String(),
			},
		)

	if err != nil {
		return nil, err
	}

	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
}

func (h *handler) getRadixBatch(batchName string) (*radixv1.RadixBatch, error) {
	return h.radixClient.RadixV1().RadixBatches(h.env.RadixDeploymentNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
}

// GarbageCollectPayloadSecrets Delete orphaned payload secrets
func (h *handler) GarbageCollectPayloadSecrets() error {
	radixBatchNameLabelExists, err := radixLabels.RequirementRadixBatchNameLabelExists()
	if err != nil {
		return err
	}
	payloadSecrets, err := h.kubeUtil.ListSecretsWithSelector(h.env.RadixDeploymentNamespace, radixBatchNameLabelExists.String())
	if err != nil {
		return err
	}
	radixBatches, err := h.getRadixBatches(radixLabels.ForComponentName(h.env.RadixComponentName))
	if err != nil {
		return err
	}
	batchNames := make(map[string]bool)
	for _, radixBatch := range radixBatches {
		batchNames[radixBatch.Name] = true
	}
	yesterday := time.Now().Add(time.Hour * -24)
	for _, payloadSecret := range payloadSecrets {
		if payloadSecret.GetCreationTimestamp().After(yesterday) {
			continue
		}
		payloadSecretBatchName, ok := payloadSecret.GetLabels()[kube.RadixBatchNameLabel]
		if !ok {
			continue
		}
		if _, ok := batchNames[payloadSecretBatchName]; !ok {
			err := h.DeleteSecret(payloadSecret)
			if err != nil {
				log.Errorf("failed deleting of an orphaned payload secret %s in the namespace %s", payloadSecret.GetName(), payloadSecret.GetNamespace())
			}
		}
	}
	return nil
}

func applyDefaultJobDescriptionProperties(jobScheduleDescription *common.JobScheduleDescription, defaultRadixJobComponentConfig *common.RadixJobComponentConfig) error {
	if jobScheduleDescription == nil || defaultRadixJobComponentConfig == nil {
		return nil
	}
	return mergo.Merge(&jobScheduleDescription.RadixJobComponentConfig, defaultRadixJobComponentConfig, mergo.WithTransformers(authTransformer))
}
