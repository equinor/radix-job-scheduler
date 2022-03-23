package defaults

const (
	//BatchScheduleDescriptionPropertyName Property name for a batch schedule description
	BatchScheduleDescriptionPropertyName = "batchScheduleDescription"
	//BatchSecretsMountPath Path to secrets, mounted to a Radix batch scheduler container
	BatchSecretsMountPath = "/mnt/secrets"
	//BatchNameEnvVarName Name of a batch
	BatchNameEnvVarName = "RADIX_BATCH_NAME"
	//BatchScheduleDescriptionPath Path to a file with batch schedule description json-file
	BatchScheduleDescriptionPath = "RADIX_BATCH_SCHEDULE_DESCRIPTION_PATH"
	//RadixBatchSchedulerContainerName Container name of the Radix batch scheduler
	RadixBatchSchedulerContainerName = "batch-scheduler"
	//RadixBatchSchedulerImage Image name of the Radix batch scheduler
	RadixBatchSchedulerImage = "radix-batch-scheduler"
	//DefaultBatchCpuLimit Default batch pod CPU limit
	DefaultBatchCpuLimit = "200m"
	//DefaultBatchMemoryLimit Default batch pod memory limit
	DefaultBatchMemoryLimit = "200Mi"
	//RadixBatchJobCountAnnotation Job count in a batch
	RadixBatchJobCountAnnotation = "radix.equinor.com\\batch-job-count"
)

//GetBatchScheduleDescriptionSecretName Get secret name for the batch schedule description
func GetBatchScheduleDescriptionSecretName(batchName string) string {
	return batchName
}
