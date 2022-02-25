package api

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//GetJob Gets a job ba its name
func (model *Model) GetJob(name string) (*batchv1.Job, error) {
	return model.KubeClient.BatchV1().Jobs(model.Env.RadixDeploymentNamespace).
		Get(
			context.TODO(),
			name,
			metav1.GetOptions{},
		)
}
