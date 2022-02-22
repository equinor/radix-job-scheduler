package defaults

import "fmt"

const (
	//BatchScheduleDescriptionPropertyName Property name for a batch schedule description
	BatchScheduleDescriptionPropertyName = "batch"
	//K8sBatchJobNameLabel A label that k8s automatically adds to a Pod created by Job and to the Job for a Batch
	K8sBatchJobNameLabel = "batch-name"
	//BatchSecretsMountPath Path to secrets, mounted to a Radix batch scheduler container
	BatchSecretsMountPath = "/mnt/secrets"
)

//GetBatchScheduleDescriptionSecretName Get secret name for the batch schedule description
func GetBatchScheduleDescriptionSecretName(batchName string) string {
	return fmt.Sprintf("%s-%s", batchName, BatchScheduleDescriptionPropertyName)
}
