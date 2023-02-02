package apiv2

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models"
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
)

var (
	authTransformer mergo.Transformers = mergoutils.CombinedTransformer{Transformers: []mergo.Transformers{mergoutils.BoolPtrTransformer{}}}
)

type handler struct {
	Kube        *kube.Kube
	Env         *models.Env
	KubeClient  kubernetes.Interface
	RadixClient radixclient.Interface
}

type Handler interface {
	//GetRadixBatches Get status of all batches
	GetRadixBatches() ([]modelsv2.RadixBatchStatus, error)
	//GetRadixBatchSingleJobs Get status of all single jobs
	GetRadixBatchSingleJobs() ([]modelsv2.RadixBatchStatus, error)
	//GetRadixBatch Get status of a batch
	GetRadixBatch(string) (*radixv1.RadixBatchStatus, error)
	//CreateRadixBatch Create a batch with parameters
	CreateRadixBatch(*models.BatchScheduleDescription) (*radixv1.RadixBatch, error)
	//CreateRadixBatchSingleJob Create a batch with single job parameters
	CreateRadixBatchSingleJob(*models.JobScheduleDescription) (*radixv1.RadixBatch, error)
	//MaintainHistoryLimit Delete outdated batches
	MaintainHistoryLimit() error
	//DeleteRadixBatch Delete a batch
	DeleteRadixBatch(string) error
}

//New Constructor of the batch handler
func New(env *models.Env, kube *kube.Kube, kubeClient kubernetes.Interface, radixClient radixclient.Interface) Handler {
	return &handler{
		Kube:        kube,
		KubeClient:  kubeClient,
		RadixClient: radixClient,
		Env:         env,
	}
}

type payloadEntry struct {
	keySelector *radixv1.PayloadSecretKeySelector
	payload     *string
}

//GetRadixBatches Get statuses of all batches
func (h *handler) GetRadixBatches() ([]modelsv2.RadixBatchStatus, error) {
	log.Debugf("Get batches for the namespace: %s", h.Env.RadixDeploymentNamespace)
	return h.getRadixBatchStatus(kube.RadixBatchTypeBatch)
}

//GetRadixBatchSingleJobs Get statuses of all single jobs
func (h *handler) GetRadixBatchSingleJobs() ([]modelsv2.RadixBatchStatus, error) {
	log.Debugf("Get sigle jobs for the namespace: %s", h.Env.RadixDeploymentNamespace)
	return h.getRadixBatchStatus(kube.RadixBatchTypeJob)
}

func (h *handler) getRadixBatchStatus(radixBatchType kube.RadixBatchType) ([]modelsv2.RadixBatchStatus, error) {
	radixBatches, err := h.getAllBatches(radixBatchType)
	if err != nil {
		return nil, err
	}
	log.Debugf("Found %v batches for namespace %s", len(radixBatches), h.Env.RadixDeploymentNamespace)

	var radixBatchStatuses []modelsv2.RadixBatchStatus
	for _, radixBatch := range radixBatches {
		radixBatchStatus := radixBatch.Status
		radixBatchStatuses = append(radixBatchStatuses, modelsv2.RadixBatchStatus{
			Name:         radixBatch.GetName(),
			CreationTime: utils.FormatTime(pointers.Ptr(radixBatch.GetCreationTimestamp())),
			Status:       radixBatchStatus,
		})
	}
	return radixBatchStatuses, nil
}

//GetRadixBatch Get status of a batch
func (h *handler) GetRadixBatch(batchName string) (*radixv1.RadixBatchStatus, error) {
	log.Debugf("get batch status for the batch %s for namespace: %s", batchName, h.Env.RadixDeploymentNamespace)
	radixBatch, err := h.getRadixBatch(batchName)
	if err != nil {
		return nil, err
	}
	return &radixBatch.Status, nil
}

//CreateRadixBatch Create a batch with parameters
func (h *handler) CreateRadixBatch(batchScheduleDescription *models.BatchScheduleDescription) (*radixv1.RadixBatch, error) {
	return h.createRadixBatchOrJob(batchScheduleDescription, kube.RadixBatchTypeBatch)
}

func (h *handler) createRadixBatchOrJob(batchScheduleDescription *models.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	namespace := h.Env.RadixDeploymentNamespace
	radixComponentName := h.Env.RadixComponentName
	radixDeploymentName := h.Env.RadixDeploymentName
	log.Debugf("create batch for namespace: %s", namespace)

	radixDeployment, err := h.RadixClient.RadixV1().RadixDeployments(namespace).
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
	return createdRadixBatch, nil
}

//CreateRadixBatchSingleJob Create a batch single job with parameters
func (h *handler) CreateRadixBatchSingleJob(jobScheduleDescription *models.JobScheduleDescription) (*radixv1.RadixBatch, error) {
	if jobScheduleDescription == nil {
		return nil, fmt.Errorf("missing expected job description")
	}
	return h.createRadixBatchOrJob(&models.BatchScheduleDescription{
		JobScheduleDescriptions:        []models.JobScheduleDescription{*jobScheduleDescription},
		DefaultRadixJobComponentConfig: nil,
	}, kube.RadixBatchTypeJob)
}

//DeleteRadixBatch Delete a batch
func (h *handler) DeleteRadixBatch(batchName string) error {
	log.Debugf("delete batch %s for namespace: %s", batchName, h.Env.RadixDeploymentNamespace)
	fg := metav1.DeletePropagationBackground
	return h.RadixClient.RadixV1().RadixBatches(h.Env.RadixDeploymentNamespace).Delete(context.Background(), batchName, metav1.DeleteOptions{PropagationPolicy: &fg})
}

//MaintainHistoryLimit Delete outdated batches
func (h *handler) MaintainHistoryLimit() error {
	radixBatches, err := h.getAllBatches("")
	if err != nil {
		return err
	}
	historyLimit := h.Env.RadixJobSchedulersPerEnvironmentHistoryLimit
	log.Debug("maintain history limit for succeeded batches")
	if err = h.maintainHistoryLimitForBatches(getSucceededBatches(radixBatches), historyLimit); err != nil {
		return err
	}
	log.Debug("maintain history limit for not succeeded batches")
	if err = h.maintainHistoryLimitForBatches(getNotSucceededBatches(radixBatches), historyLimit); err != nil {
		return err
	}
	log.Debug("maintain history limit for succeeded single jobs")
	if err = h.maintainHistoryLimitForBatches(getSucceededSingleJobs(radixBatches), historyLimit); err != nil {
		return err
	}
	log.Debug("maintain history limit for not succeeded single jobs")
	if err = h.maintainHistoryLimitForBatches(getNotSucceededSingleJobs(radixBatches), historyLimit); err != nil {
		return err
	}
	return nil
}

func getNotSucceededBatches(radixBatches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	return slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return len(radixBatch.Spec.Jobs) > 0 && isRadixBatchNotSucceeded(radixBatch)
	})
}

func getSucceededBatches(radixBatches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	return slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return len(radixBatch.Spec.Jobs) > 0 && isRadixBatchSucceeded(radixBatch)
	})
}

func getNotSucceededSingleJobs(radixBatches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	return slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return len(radixBatch.Spec.Jobs) < 2 && isRadixBatchNotSucceeded(radixBatch)
	})
}

func getSucceededSingleJobs(radixBatches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	return slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return len(radixBatch.Spec.Jobs) < 2 && isRadixBatchSucceeded(radixBatch)
	})
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

func (h *handler) maintainHistoryLimitForBatches(radixBatches []*radixv1.RadixBatch, historyLimit int) error {
	numToDelete := len(radixBatches) - historyLimit
	if numToDelete <= 0 {
		log.Debug("no history batches to delete")
		return nil
	}
	log.Debugf("history batches to delete: %v", numToDelete)

	sortedBatches := sortRJSchByCompletionTimeAsc(radixBatches)
	for i := 0; i < numToDelete; i++ {
		batch := sortedBatches[i]
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

func (h *handler) createBatch(namespace, appName, radixDeploymentName string, radixJobComponent *radixv1.RadixDeployJobComponent, batchScheduleDescription *models.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	batchName := generateBatchName(radixJobComponent.GetName())
	radixJobComponentName := radixJobComponent.GetName()
	batchJobs, err := h.buildRadixBatchJobs(namespace, appName, radixJobComponentName, batchName, batchScheduleDescription)
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
			Jobs: batchJobs,
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  radixJobComponentName,
			},
		},
	}
	return h.RadixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), &radixBatch,
		metav1.CreateOptions{})
}

func (h *handler) buildRadixBatchJobs(namespace, appName, batchName, radixJobComponentName string, batchScheduleDescription *models.BatchScheduleDescription) ([]radixv1.RadixBatchJob, error) {
	var batchJobs []radixv1.RadixBatchJob
	var payloadEntries []payloadEntry
	for i, jobScheduleDescription := range batchScheduleDescription.JobScheduleDescriptions {
		jobName := fmt.Sprintf("job%d", i)
		job, err := buildRadixBatchJob(jobName, &jobScheduleDescription, batchScheduleDescription.DefaultRadixJobComponentConfig)
		if err != nil {
			return nil, err
		}
		keySelector := &radixv1.PayloadSecretKeySelector{
			Key: job.Name,
		}
		payloadEntries = append(payloadEntries, payloadEntry{
			keySelector: keySelector,
			payload:     &jobScheduleDescription.Payload,
		})
		job.PayloadSecretRef = keySelector
		batchJobs = append(batchJobs, *job)
	}
	jobs, err := h.createPayloadSecrets(namespace, appName, batchName, radixJobComponentName, payloadEntries)
	if err != nil {
		return jobs, err
	}
	return batchJobs, nil
}

func (h *handler) createPayloadSecrets(namespace string, appName string, batchName string, radixJobComponentName string, payloadEntries []payloadEntry) ([]radixv1.RadixBatchJob, error) {
	accumulatedSecretSize := 0
	const maxSecretSize = 1024 * 1024 * 512 //0.5MB
	const entryAuxDataSize = 128
	secretCount := 0
	payloadsSecret := buildSecret(appName, radixJobComponentName, batchName, secretCount)
	for ind, entry := range payloadEntries {
		payload := []byte(*entry.payload)
		payloadBase64 := base64.RawStdEncoding.EncodeToString(payload)
		secretEntrySize := len(payloadBase64) + len(entry.keySelector.Key) + entryAuxDataSize
		if accumulatedSecretSize+secretEntrySize > maxSecretSize {
			if accumulatedSecretSize == 0 {
				return nil, fmt.Errorf("payload is too large in the job %d - its base64 size is %d, but it expected to be less then %d", ind, secretEntrySize, maxSecretSize)
			}
			_, err := h.Kube.ApplySecret(namespace, payloadsSecret)
			if err != nil {
				return nil, err
			}
			secretCount = secretCount + 1
			payloadsSecret = buildSecret(appName, radixJobComponentName, batchName, secretCount)
		}
		payloadsSecret.Data[entry.keySelector.Key] = payload
		accumulatedSecretSize = accumulatedSecretSize + secretEntrySize
	}
	return nil, nil
}

func buildSecret(appName, radixJobComponentName, batchName string, secretIndex int) *corev1.Secret {
	secretName := fmt.Sprintf("%s-payloads-%d", batchName, secretIndex)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(radixJobComponentName),
				radixLabels.ForBatchJobName(batchName),
			),
		},
		Data: make(map[string][]byte),
	}
}

func buildRadixBatchJob(jobName string, jobScheduleDescription *models.JobScheduleDescription, defaultJobScheduleDescription *models.RadixJobComponentConfig) (*radixv1.RadixBatchJob, error) {
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
		Stop:             nil,
	}, nil
}

func (h *handler) getAllBatches(radixBatchType kube.RadixBatchType) ([]*radixv1.RadixBatch, error) {
	radixBatchList, err := h.RadixClient.
		RadixV1().
		RadixBatches(h.Env.RadixDeploymentNamespace).
		List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: radixLabels.Merge(
					radixLabels.ForComponentName(h.Env.RadixComponentName),
					radixLabels.ForBatchType(radixBatchType),
				).String(),
			},
		)

	if err != nil {
		return nil, err
	}

	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
}

func (h *handler) getRadixBatch(batchName string) (*radixv1.RadixBatch, error) {
	return h.RadixClient.RadixV1().RadixBatches(h.Env.RadixDeploymentNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
}

func applyDefaultJobDescriptionProperties(jobScheduleDescription *models.JobScheduleDescription, defaultRadixJobComponentConfig *models.RadixJobComponentConfig) error {
	if jobScheduleDescription == nil || defaultRadixJobComponentConfig == nil {
		return nil
	}
	return mergo.Merge(&jobScheduleDescription.RadixJobComponentConfig, defaultRadixJobComponentConfig, mergo.WithTransformers(authTransformer))
}
