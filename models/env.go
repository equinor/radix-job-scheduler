package models

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
)

// Env instance variables
type Env struct {
	UseSwagger                                   bool
	RadixDNSZone                                 string
	RadixContainerRegistry                       string
	RadixClusterName                             string
	RadixActiveClusterEgressIps                  string
	RadixAppName                                 string
	RadixEnvironment                             string
	RadixComponentName                           string
	RadixDeploymentName                          string
	RadixDeploymentNamespace                     string
	RadixJobSchedulersPerEnvironmentHistoryLimit int
	RadixPort                                    string
	RadixDefaultCpuLimit                         string
	RadixDefaultMemoryLimit                      string
	//RadixBatchSchedulerImageFullName The name of the Radix batch cheduler image, including comtainer repository and tag
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
		radixDNSZone                                 = strings.TrimSpace(os.Getenv(defaults.RadixDNSZoneEnvironmentVariable))
		radixClusterName                             = strings.TrimSpace(os.Getenv(defaults.ClusternameEnvironmentVariable))
		radixActiveClusterEgressIps                  = strings.TrimSpace(os.Getenv(defaults.RadixActiveClusterEgressIpsEnvironmentVariable))
		radixAppName                                 = strings.TrimSpace(os.Getenv(defaults.RadixAppEnvironmentVariable))
		radixEnv                                     = strings.TrimSpace(os.Getenv(defaults.EnvironmentnameEnvironmentVariable))
		radixComponentName                           = strings.TrimSpace(os.Getenv(defaults.RadixComponentEnvironmentVariable))
		radixDeployment                              = strings.TrimSpace(os.Getenv(defaults.RadixDeploymentEnvironmentVariable))
		radixJobSchedulersPerEnvironmentHistoryLimit = strings.TrimSpace(os.Getenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT"))
		radixPorts                                   = strings.TrimSpace(os.Getenv(defaults.RadixPortsEnvironmentVariable))
		radixContainerRegistry                       = strings.TrimSpace(os.Getenv(defaults.ContainerRegistryEnvironmentVariable))
	)
	env := Env{
		RadixDNSZone:                radixDNSZone,
		RadixAppName:                radixAppName,
		RadixContainerRegistry:      radixContainerRegistry,
		RadixClusterName:            radixClusterName,
		RadixActiveClusterEgressIps: radixActiveClusterEgressIps,
		RadixEnvironment:            radixEnv,
		RadixComponentName:          radixComponentName,
		RadixDeploymentName:         radixDeployment,
		RadixDeploymentNamespace:    utils.GetEnvironmentNamespace(radixAppName, radixEnv),
		UseSwagger:                  useSwagger,
		RadixJobSchedulersPerEnvironmentHistoryLimit: 10,
	}
	setPort(radixPorts, &env)
	setHistoryLimit(radixJobSchedulersPerEnvironmentHistoryLimit, &env)
	return &env
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
