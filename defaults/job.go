package defaults

import "fmt"

const (
	//JobPayloadPropertyName Property name for payload
	JobPayloadPropertyName = "payload"
	//K8sJobNameLabel A label that k8s automatically adds to a Pod created by a Job
	K8sJobNameLabel = "job-name"
)

//GetPayloadSecretName Get secret name for the payload
func GetPayloadSecretName(jobName string) string {
	return fmt.Sprintf("%s-%s", jobName, JobPayloadPropertyName)
}
