package v1

import (
	"context"

	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
)

type Handler struct {
	Kube                    *kube.Kube
	Config                  *models.Config
	HandlerApiV2            apiv2.Handler
	RadixDeployJobComponent *radixv1.RadixDeployJobComponent
}

func (h *Handler) ListRadixBatches(ctx context.Context, radixBatchType kube.RadixBatchType) ([]radixv1.RadixBatch, error) {
	labels := []map[string]string{radixlabels.ForComponentName(h.RadixDeployJobComponent.GetName())}
	if len(radixBatchType) > 0 {
		labels = append(labels, radixlabels.ForBatchType(radixBatchType))
	}

	return internal.ListRadixBatches(ctx, h.Config.RadixDeploymentNamespace, h.Kube.RadixClient(), labels...)
}
