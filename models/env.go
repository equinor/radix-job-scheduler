package models

import (
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
)

// Env instance variables
type Env struct {
	UseSwagger                                   bool
	RadixDeployment                              string
	RadixDeploymentNamespace                     string
	RadixJobSchedulersPerEnvironmentHistoryLimit int
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
		radixDeployment                              = strings.TrimSpace(os.Getenv("RADIX_DEPLOYMENT"))
		radixDeploymentNamespace                     = strings.TrimSpace(os.Getenv("RADIX_DEPLOYMENT_NAMESPACE"))
		radixJobSchedulersPerEnvironmentHistoryLimit = strings.TrimSpace(os.Getenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT"))
	)

	env := Env{
		RadixDeployment:          radixDeployment,
		RadixDeploymentNamespace: radixDeploymentNamespace,
		UseSwagger:               useSwagger,
		RadixJobSchedulersPerEnvironmentHistoryLimit: 10,
	}
	if len(radixJobSchedulersPerEnvironmentHistoryLimit) > 0 {
		if historyLimit, err := strconv.Atoi(radixJobSchedulersPerEnvironmentHistoryLimit); err == nil && historyLimit > 0 {
			env.RadixJobSchedulersPerEnvironmentHistoryLimit = historyLimit
		}
	}
	return &env
}

func envVarIsTrueOrYes(envVar string) bool {
	return strings.EqualFold(envVar, "true") || strings.EqualFold(envVar, "yes")
}
