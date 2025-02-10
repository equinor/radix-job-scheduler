package v1

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	apierrors "github.com/equinor/radix-job-scheduler/api/errors"
	internalactions "github.com/equinor/radix-job-scheduler/internal/actions"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/query"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type BatchStatusBuilderFunc func(rb radixv1.RadixBatch, batchStatusRules []radixv1.BatchStatusRule, events []corev1.Event) modelsv1.BatchStatus

type Handler struct {
	Kube                    *kube.Kube
	Config                  *config.Config
	RadixDeployJobComponent *radixv1.RadixDeployJobComponent
	BatchStatusBuilder      BatchStatusBuilderFunc
}

func (h *Handler) ListRadixBatches(ctx context.Context, types ...kube.RadixBatchType) ([]radixv1.RadixBatch, error) {
	selector := radixlabels.ForComponentName(h.RadixDeployJobComponent.GetName()).AsSelector()
	if len(types) > 0 {
		req, err := labels.NewRequirement(kube.RadixBatchTypeLabel, selection.In, slice.Map(types, func(t kube.RadixBatchType) string { return string(t) }))
		if err != nil {
			return nil, fmt.Errorf("failed to build batch type requirement: %w", err)
		}
		selector = selector.Add(*req)
	}

	return query.ListRadixBatches(ctx, h.Config.RadixDeploymentNamespace, h.Kube.RadixClient(), selector)
}

func (h *Handler) GetRadixBatch(ctx context.Context, batchName string) (*radixv1.RadixBatch, error) {
	// TODO: We must ensure that only RadixBatches for the correct jobcomponent is returned,
	// for example by using a metav1.SingleObject() with List command, and returning a new kuberrors.NewNotFound if no items returned
	return query.GetRadixBatch(ctx, h.Kube.RadixClient(), h.Config.RadixDeploymentNamespace, batchName)
}

func (h *Handler) UpdateRadixBatch(ctx context.Context, rb *radixv1.RadixBatch) (*radixv1.RadixBatch, error) {
	return h.Kube.RadixClient().RadixV1().RadixBatches(rb.GetNamespace()).Update(ctx, rb, metav1.UpdateOptions{})
}

func (h *Handler) CreateNewBatchFromRequest(ctx context.Context, request common.BatchScheduleDescription, batchType kube.RadixBatchType) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create job for namespace: %s", h.Config.RadixDeploymentNamespace)

	rb, secrets, err := internalactions.BuildRadixBatchResources(
		request,
		batchType,
		h.Config.RadixAppName,
		h.Config.RadixDeploymentName,
		*h.RadixDeployJobComponent,
	)
	if err != nil {
		if errors.Is(err, internalactions.ErrPayloadTooLarge) {
			return nil, apierrors.NewBadRequestError("payload too large", err)
		}
		if errors.Is(err, internalactions.ErrPayloadPathNotConfiguredForJob) {
			return nil, apierrors.NewBadRequestError("payload path is not configured for job", err)
		}
		return nil, apierrors.NewFromError(err)
	}

	secretClient := h.Kube.KubeClient().CoreV1().Secrets(h.Config.RadixDeploymentNamespace)
	for _, secret := range secrets {
		if _, err := secretClient.Create(ctx, &secret, metav1.CreateOptions{}); err != nil {
			return nil, apierrors.NewFromError(err)
		}
	}
	rb, err = h.Kube.RadixClient().RadixV1().RadixBatches(h.Config.RadixDeploymentNamespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		return nil, apierrors.NewFromError(err)
	}

	modelMapperFunc, err := h.CreateBatchStatusMapper(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create model mapper: %w", err)
	}
	batchStatus := modelMapperFunc(*rb)
	return &batchStatus, nil
}

func (h *Handler) CreateBatchStatusMapper(ctx context.Context) (func(rb radixv1.RadixBatch) modelsv1.BatchStatus, error) {
	builderFunc := modelsv1.BuildBatchStatus
	if h.BatchStatusBuilder != nil {
		builderFunc = h.BatchStatusBuilder
	}

	events, err := h.listEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	rules := h.getBatchStatusRules()

	mapFunc := func(rb radixv1.RadixBatch) modelsv1.BatchStatus {
		return builderFunc(rb, rules, events)
	}

	return mapFunc, nil
}

func (h *Handler) listEvents(ctx context.Context) ([]corev1.Event, error) {
	return query.ListEvents(ctx, h.Config.RadixDeploymentNamespace, h.Kube.KubeClient())
}

func (h *Handler) getBatchStatusRules() []radixv1.BatchStatusRule {
	if h.RadixDeployJobComponent == nil {
		return nil
	}
	return h.RadixDeployJobComponent.BatchStatusRules
}
