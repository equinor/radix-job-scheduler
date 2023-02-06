package apiv2

import (
	"context"
	"encoding/base64"
	"fmt"
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
	// GetCompletedRadixBatchesSortedByCompletionTimeAsc Gets completed RadixBatch lists for Env.RadixComponentName
	GetCompletedRadixBatchesSortedByCompletionTimeAsc() (*CompletedRadixBatches, error)
}

// CompletedRadixBatches Completed RadixBatch lists
type CompletedRadixBatches struct {
	SucceededRadixBatches    []*radixv1.RadixBatch
	NotSucceededRadixBatches []*radixv1.RadixBatch
	SucceededSingleJobs      []*radixv1.RadixBatch
	NotSucceededSingleJobs   []*radixv1.RadixBatch
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

type radixBatchJobWithDescription struct {
	radixBatchJob          *radixv1.RadixBatchJob
	jobScheduleDescription *models.JobScheduleDescription
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
	radixBatches, err := h.getRadixBatches(
		radixLabels.ForComponentName(h.Env.RadixComponentName),
		radixLabels.ForBatchType(radixBatchType),
	)
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
	completedRadixBatches, _ := h.GetCompletedRadixBatchesSortedByCompletionTimeAsc()

	historyLimit := h.Env.RadixJobSchedulersPerEnvironmentHistoryLimit
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
	return commonErrors.Concat(errs)
}

// GetCompletedRadixBatchesSortedByCompletionTimeAsc Gets completed RadixBatch lists for Env.RadixComponentName
func (h *handler) GetCompletedRadixBatchesSortedByCompletionTimeAsc() (*CompletedRadixBatches, error) {
	radixBatches, err := h.getRadixBatches(radixLabels.ForComponentName(h.Env.RadixComponentName))
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

func getNotSucceededRadixBatches(radixBatches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	return slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchNotSucceeded(radixBatch)
	})
}

func getSucceededRadixBatches(radixBatches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	return slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchSucceeded(radixBatch)
	})
}

func getNotSucceededSingleJobs(radixBatches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	return slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchNotSucceeded(radixBatch)
	})
}

func getSucceededSingleJobs(radixBatches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	return slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchSucceeded(radixBatch)
	})
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

func (h *handler) maintainHistoryLimitForBatches(radixBatchesSortedByCompletionTimeAsc []*radixv1.RadixBatch, historyLimit int) error {
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

func (h *handler) createBatch(namespace, appName, radixDeploymentName string, radixJobComponent *radixv1.RadixDeployJobComponent, batchScheduleDescription *models.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	batchName := generateBatchName(radixJobComponent.GetName())
	radixJobComponentName := radixJobComponent.GetName()
	batchJobs, err := h.buildRadixBatchJobs(namespace, appName, radixJobComponentName, batchName, batchScheduleDescription, radixJobComponent.Payload)
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

func (h *handler) buildRadixBatchJobs(namespace, appName, radixJobComponentName, batchName string, batchScheduleDescription *models.BatchScheduleDescription, radixJobComponentPayload *radixv1.RadixJobComponentPayload) ([]radixv1.RadixBatchJob, error) {
	var radixBatchJobWithDescriptions []radixBatchJobWithDescription
	var errs []error
	for i, jobScheduleDescription := range batchScheduleDescription.JobScheduleDescriptions {
		jobName := fmt.Sprintf("job%d", i)
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
	hasPayloadPath := radixJobComponentPayload != nil && len(radixJobComponentPayload.Path) > 0
	err := h.createRadixBatchJobPayloadSecrets(namespace, appName, radixJobComponentName, batchName, radixBatchJobWithDescriptions, hasPayloadPath)
	if err != nil {
		return nil, err
	}
	var radixBatchJobs []radixv1.RadixBatchJob
	for _, item := range radixBatchJobWithDescriptions {
		radixBatchJobs = append(radixBatchJobs, *item.radixBatchJob)
	}
	return radixBatchJobs, nil
}

func (h *handler) createRadixBatchJobPayloadSecrets(namespace, appName, radixJobComponentName, batchName string, radixJobWithPayloadEntries []radixBatchJobWithDescription, hasPayloadPath bool) error {
	accumulatedSecretSize := 0
	var payloadSecrets []*corev1.Secret
	payloadsSecret := buildSecret(appName, radixJobComponentName, batchName, 0)
	payloadSecrets = append(payloadSecrets, payloadsSecret)
	var errs []error
	for i, radixJobWithDescriptions := range radixJobWithPayloadEntries {
		payloadArray := []byte(strings.TrimSpace(radixJobWithDescriptions.jobScheduleDescription.Payload))
		if len(payloadArray) == 0 {
			log.Debugf("no payload in the job #%d", i)
			continue
		}
		if !hasPayloadPath {
			errs = append(errs, fmt.Errorf("missing an expected paylod path, but there is a payload in the job #%d", i))
			continue
		}
		payloadBase64 := base64.RawStdEncoding.EncodeToString(payloadArray)
		secretEntrySize := len(payloadBase64) + len(radixJobWithDescriptions.radixBatchJob.Name) + payloadSecretEntryAuxDataSize
		if payloadSecretAuxDataSize+accumulatedSecretSize+secretEntrySize > maxPayloadSecretSize {
			if accumulatedSecretSize == 0 {
				return fmt.Errorf("payload is too large in the job #%d - its base64 size is %d bytes, but it is expected to be less then %d bytes", i, secretEntrySize, maxPayloadSecretSize)
			}
			payloadsSecret = buildSecret(appName, radixJobComponentName, batchName, len(payloadSecrets))
			payloadSecrets = append(payloadSecrets, payloadsSecret)
			accumulatedSecretSize = 0
		}
		payloadsSecret.Data[radixJobWithDescriptions.radixBatchJob.Name] = payloadArray
		accumulatedSecretSize = accumulatedSecretSize + secretEntrySize
		radixJobWithDescriptions.radixBatchJob.PayloadSecretRef = &radixv1.PayloadSecretKeySelector{
			LocalObjectReference: radixv1.LocalObjectReference{Name: payloadsSecret.GetName()},
			Key:                  radixJobWithDescriptions.radixBatchJob.Name,
		}
	}
	if len(errs) > 0 {
		return commonErrors.Concat(errs)
	}
	return h.createSecrets(namespace, payloadSecrets)
}

func (h *handler) createSecrets(namespace string, secrets []*corev1.Secret) error {
	var errs []error
	for _, secret := range secrets {
		if secret.Data == nil || len(secret.Data) == 0 {
			continue //if Data is empty, the secret is not used in any jobs
		}
		_, err := h.Kube.KubeClient().CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return commonErrors.Concat(errs)
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

func (h *handler) getRadixBatches(labels ...map[string]string) ([]*radixv1.RadixBatch, error) {
	radixBatchList, err := h.RadixClient.
		RadixV1().
		RadixBatches(h.Env.RadixDeploymentNamespace).
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
	return h.RadixClient.RadixV1().RadixBatches(h.Env.RadixDeploymentNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
}

func applyDefaultJobDescriptionProperties(jobScheduleDescription *models.JobScheduleDescription, defaultRadixJobComponentConfig *models.RadixJobComponentConfig) error {
	if jobScheduleDescription == nil || defaultRadixJobComponentConfig == nil {
		return nil
	}
	return mergo.Merge(&jobScheduleDescription.RadixJobComponentConfig, defaultRadixJobComponentConfig, mergo.WithTransformers(authTransformer))
}
