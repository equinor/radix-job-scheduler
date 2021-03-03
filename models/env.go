package models

import (
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

// Env instance variables
type Env struct {
	UseSwagger               bool
	RadixDeployment          string
	RadixDeploymentNamespace string
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
		useSwagger               = envVarIsTrueOrYes(os.Getenv("USE_SWAGGER"))
		radixDeployment          = strings.TrimSpace(os.Getenv("RADIX_DEPLOYMENT"))
		radixDeploymentNamespace = strings.TrimSpace(os.Getenv("RADIX_DEPLOYMENT_NAMESPACE"))
	)
	return &Env{
		RadixDeployment:          radixDeployment,
		RadixDeploymentNamespace: radixDeploymentNamespace,
		UseSwagger:               useSwagger,
	}
}

func envVarIsTrueOrYes(envVar string) bool {
	return strings.EqualFold(envVar, "true") || strings.EqualFold(envVar, "yes")
}
