package models

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

// Config instance variables
type Config struct {
	UseSwagger                                   bool
	RadixComponentName                           string
	RadixDeploymentName                          string
	RadixDeploymentNamespace                     string
	RadixJobSchedulersPerEnvironmentHistoryLimit int
	RadixPort                                    string
	LogLevel                                     string
}

// NewConfigFromEnv Constructor
func NewConfigFromEnv() *Config {
	var (
		useSwagger, _                                = strconv.ParseBool(os.Getenv("USE_SWAGGER"))
		radixAppName                                 = strings.TrimSpace(os.Getenv(defaults.RadixAppEnvironmentVariable))
		radixEnv                                     = strings.TrimSpace(os.Getenv(defaults.EnvironmentnameEnvironmentVariable))
		radixComponentName                           = strings.TrimSpace(os.Getenv(defaults.RadixComponentEnvironmentVariable))
		radixDeployment                              = strings.TrimSpace(os.Getenv(defaults.RadixDeploymentEnvironmentVariable))
		radixJobSchedulersPerEnvironmentHistoryLimit = strings.TrimSpace(os.Getenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT"))
		radixPorts                                   = strings.TrimSpace(os.Getenv(defaults.RadixPortsEnvironmentVariable))
		logLevel                                     = os.Getenv("LOG_LEVEL")
	)
	cfg := Config{
		RadixComponentName:       radixComponentName,
		RadixDeploymentName:      radixDeployment,
		RadixDeploymentNamespace: utils.GetEnvironmentNamespace(radixAppName, radixEnv),
		UseSwagger:               useSwagger,
		RadixJobSchedulersPerEnvironmentHistoryLimit: 10,
		LogLevel: logLevel,
	}
	setPort(radixPorts, &cfg)
	setHistoryLimit(radixJobSchedulersPerEnvironmentHistoryLimit, &cfg)
	return &cfg
}

func setHistoryLimit(radixJobSchedulersPerEnvironmentHistoryLimit string, env *Config) {
	if len(radixJobSchedulersPerEnvironmentHistoryLimit) > 0 {
		if historyLimit, err := strconv.Atoi(radixJobSchedulersPerEnvironmentHistoryLimit); err == nil && historyLimit > 0 {
			env.RadixJobSchedulersPerEnvironmentHistoryLimit = historyLimit
		}
	}
}

func setPort(radixPorts string, env *Config) {
	radixPorts = strings.ReplaceAll(radixPorts, "(", "")
	radixPorts = strings.ReplaceAll(radixPorts, ")", "")
	ports := strings.Split(radixPorts, ",")
	if len(ports) > 0 {
		env.RadixPort = ports[0]
		return
	}
	panic(fmt.Errorf("RADIX_PORTS not set"))
}
