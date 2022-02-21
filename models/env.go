package models

// Env instance variables
type Env struct {
	UseSwagger                                   bool
	RadixAppName                                 string
	RadixComponentName                           string
	RadixDeploymentName                          string
	RadixDeploymentNamespace                     string
	RadixJobSchedulersPerEnvironmentHistoryLimit int
	RadixPort                                    string
}
