package defaults

import "fmt"

const (
	//BatchScheduleDescriptionPropertyName Property name for a batch schedule description
	BatchScheduleDescriptionPropertyName = "batch"
	//K8sBatchJobNameLabel A label that k8s automatically adds to a Pod created by Job and to the Job for a Batch
	K8sBatchJobNameLabel = "batch-name"
	//BatchSecretsMountPath Path to secrets, mounted to a Radix batch scheduler container
	BatchSecretsMountPath = "/mnt/secrets"
	//BatchNameEnvVarName Name of a batch
	BatchNameEnvVarName = "RADIX_BATCH_NAME"
	//BatchScheduleDescriptionPath Path to a file with batch schedule description json-file
	BatchScheduleDescriptionPath = "RADIX_BATCH_SCHEDULE_DESCRIPTION_PATH"
)

//GetBatchScheduleDescriptionSecretName Get secret name for the batch schedule description
func GetBatchScheduleDescriptionSecretName(batchName string) string {
	return fmt.Sprintf("%s-%s", batchName, BatchScheduleDescriptionPropertyName)
}
