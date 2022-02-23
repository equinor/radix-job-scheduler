package models

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	schedulerDefaults "github.com/equinor/radix-job-scheduler/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
)

// Env instance variables
type Env struct {
	UseSwagger                                   bool
	RadixAppName                                 string
	RadixComponentName                           string
	RadixDeploymentName                          string
	RadixDeploymentNamespace                     string
	RadixJobSchedulersPerEnvironmentHistoryLimit int
	RadixPort                                    string
	//RadixBatchSchedulerImageFullName The name of the Radix batch cheduler image,
	//including comtainer repository and tag
	RadixBatchSchedulerImageFullName string
}

// NewEnv Constructor
func NewEnv() *Env {
	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
	var (
		useSwagger                                   = envVarIsTrueOrYes(os.Getenv("USE_SWAGGER"))
		radixAppName                                 = strings.TrimSpace(os.Getenv("RADIX_APP"))
		radixEnv                                     = strings.TrimSpace(os.Getenv("RADIX_ENVIRONMENT"))
		radixComponentName                           = strings.TrimSpace(os.Getenv("RADIX_COMPONENT"))
		radixDeployment                              = strings.TrimSpace(os.Getenv("RADIX_DEPLOYMENT"))
		radixJobSchedulersPerEnvironmentHistoryLimit = strings.TrimSpace(os.Getenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT"))
		radixPorts                                   = strings.TrimSpace(os.Getenv("RADIX_PORTS"))
		containerRegistryEnvironmentVariable         = strings.TrimSpace(os.Getenv("RADIX_CONTAINER_REGISTRY"))
	)
	env := Env{
		RadixAppName:             radixAppName,
		RadixComponentName:       radixComponentName,
		RadixDeploymentName:      radixDeployment,
		RadixDeploymentNamespace: utils.GetEnvironmentNamespace(radixAppName, radixEnv),
		UseSwagger:               useSwagger,
		RadixJobSchedulersPerEnvironmentHistoryLimit: 10,
		RadixBatchSchedulerImageFullName: getRadixBatchSchedulerImageFullName(
			containerRegistryEnvironmentVariable, radixEnv),
	}
	setPort(radixPorts, &env)
	setHistoryLimit(radixJobSchedulersPerEnvironmentHistoryLimit, &env)
	return &env
}

func getRadixBatchSchedulerImageFullName(containerRegistry, radixEnv string) string {
	tag := commonUtils.TernaryString(strings.EqualFold(radixEnv, "prod"), "release-latest", "main-latest")
	return fmt.Sprintf("%s/%s:%s", containerRegistry, schedulerDefaults.RadixBatchSchedulerImage, tag)
}

func setHistoryLimit(radixJobSchedulersPerEnvironmentHistoryLimit string, env *Env) {
	if len(radixJobSchedulersPerEnvironmentHistoryLimit) > 0 {
		if historyLimit, err := strconv.Atoi(radixJobSchedulersPerEnvironmentHistoryLimit); err == nil && historyLimit > 0 {
			env.RadixJobSchedulersPerEnvironmentHistoryLimit = historyLimit
		}
	}
}

func setPort(radixPorts string, env *Env) {
	radixPorts = strings.ReplaceAll(radixPorts, "(", "")
	radixPorts = strings.ReplaceAll(radixPorts, ")", "")
	ports := strings.Split(radixPorts, ",")
	if len(ports) > 0 {
		env.RadixPort = ports[0]
		return
	}
	panic(fmt.Errorf("RADIX_PORTS not set"))
}

func envVarIsTrueOrYes(envVar string) bool {
	return strings.EqualFold(envVar, "true") || strings.EqualFold(envVar, "yes")
}
