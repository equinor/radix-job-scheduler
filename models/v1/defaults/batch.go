package defaultsv1

const (
	//BatchScheduleDescriptionPropertyName Property name for a batch schedule description
	BatchScheduleDescriptionPropertyName = "batchScheduleDescription"
	//RadixBatchSchedulerImage Image name of the Radix batch scheduler
	RadixBatchSchedulerImage = "radix-batch-scheduler"
	//DefaultBatchCpuLimit Default batch pod CPU limit
	DefaultBatchCpuLimit = "200m"
	//DefaultBatchMemoryLimit Default batch pod memory limit
	DefaultBatchMemoryLimit = "200Mi"
)

//GetBatchScheduleDescriptionSecretName Get secret name for the batch schedule description
func GetBatchScheduleDescriptionSecretName(batchName string) string {
	return batchName
}
