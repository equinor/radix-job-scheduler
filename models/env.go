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
	RadixComponentName                           string
	RadixDeploymentName                          string
	RadixDeploymentNamespace                     string
	RadixJobSchedulersPerEnvironmentHistoryLimit int
	RadixPort                                    string
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
		radixAppName                                 = strings.TrimSpace(os.Getenv(defaults.RadixAppEnvironmentVariable))
		radixEnv                                     = strings.TrimSpace(os.Getenv(defaults.EnvironmentnameEnvironmentVariable))
		radixComponentName                           = strings.TrimSpace(os.Getenv(defaults.RadixComponentEnvironmentVariable))
		radixDeployment                              = strings.TrimSpace(os.Getenv(defaults.RadixDeploymentEnvironmentVariable))
		radixJobSchedulersPerEnvironmentHistoryLimit = strings.TrimSpace(os.Getenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT"))
		radixPorts                                   = strings.TrimSpace(os.Getenv(defaults.RadixPortsEnvironmentVariable))
	)
	env := Env{
		RadixComponentName:       radixComponentName,
		RadixDeploymentName:      radixDeployment,
		RadixDeploymentNamespace: utils.GetEnvironmentNamespace(radixAppName, radixEnv),
		UseSwagger:               useSwagger,
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
