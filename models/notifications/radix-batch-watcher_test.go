package notifications

import (
	"context"
	"testing"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_RadixBatchWatcher(t *testing.T) {
	type fields struct {
		existingRadixBatch *radixv1.RadixBatch
		modifyRadixBatch   func(*radixv1.RadixBatch) *radixv1.RadixBatch
	}
	type args struct {
		getNotifier func(*gomock.Controller) Notifier
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "No batch, no notifications",
			fields: fields{
				existingRadixBatch: nil,
			},
			args: args{
				getNotifier: func(ctrl *gomock.Controller) Notifier {
					return NewMockNotifier(ctrl)
				},
			},
		},
		{
			name: "Created batch, gets notifications",
			fields: fields{
				existingRadixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec: radixv1.RadixBatchSpec{
						Jobs: []radixv1.RadixBatchJob{
							{Name: "job1"},
						},
					},
				},
				modifyRadixBatch: func(radixBatch *radixv1.RadixBatch) *radixv1.RadixBatch {
					radixBatch.Status.Condition.Type = radixv1.BatchConditionTypeWaiting
					return radixBatch
				},
			},
			args: args{
				getNotifier: func(ctrl *gomock.Controller) Notifier {
					notifier := NewMockNotifier(ctrl)
					matcher := newRadixBatchMatcher(func(radixBatch *radixv1.RadixBatch) bool {
						return radixBatch.Name == "batch1"
					})
					notifier.EXPECT().Notify(matcher,
						nil, gomock.Any()).Times(1)
					return notifier
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			radixClient := radixclientfake.NewSimpleClientset()
			namespace := "app-qa"
			var createdRadixBatch *radixv1.RadixBatch
			var err error
			if tt.fields.existingRadixBatch != nil {
				createdRadixBatch, err = radixClient.RadixV1().RadixBatches(namespace).Create(context.Background(), tt.fields.existingRadixBatch, metav1.CreateOptions{})
				if err != nil {
					assert.Fail(t, err.Error())
					return
				}
			}

			ctrl := gomock.NewController(t)
			watcher, err := NewRadixBatchWatcher(radixClient, namespace, tt.args.getNotifier(ctrl))
			if err != nil {
				assert.Fail(t, err.Error())
				watcher.Stop <- struct{}{}
				return
			}
			assert.False(t, commonUtils.IsNil(watcher))
			time.Sleep(time.Second * 1)

			if tt.fields.existingRadixBatch != nil && createdRadixBatch != nil && tt.fields.modifyRadixBatch != nil {
				_, err := radixClient.RadixV1().RadixBatches(namespace).Update(context.Background(), tt.fields.modifyRadixBatch(createdRadixBatch), metav1.UpdateOptions{})
				if err != nil {
					assert.Fail(t, err.Error())
					return
				}
			}
			time.Sleep(time.Second * 5)
			watcher.Stop <- struct{}{}
			ctrl.Finish()
		})
	}
}

func newRadixBatchMatcher(matches func(*radixv1.RadixBatch) bool) *radixBatchMatcher {
	return &radixBatchMatcher{matches: matches}
}

type radixBatchMatcher struct {
	matches func(*radixv1.RadixBatch) bool
}

func (m *radixBatchMatcher) Matches(x interface{}) bool {
	radixBatch := x.(*radixv1.RadixBatch)
	return m.matches(radixBatch)
}

func (m *radixBatchMatcher) String() string {
	return "radixBatch matcher"
}
