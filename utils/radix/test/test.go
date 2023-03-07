package test

import (
	"github.com/equinor/radix-common/utils/numbers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

func GetRadixDeploymentWithRadixJobComponent(appName, environment, componentName string, port int32) *radixv1.RadixDeployment {
	return utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithImageTag("image-tag").
		WithEnvironment(environment).
		WithJobComponent(utils.NewDeployJobComponentBuilder().
			WithImage("radixdev.azurecr.io/some-image:image-tag").
			WithName(componentName).
			WithPort("http", port).
			WithSchedulerPort(numbers.Int32Ptr(9090))).BuildRD()
}

func GetRadixApplicationWithRadixJobComponent(appName, environment, envBranch, jobComponentName string, port int32, notifications *radixv1.Notifications) *radixv1.RadixApplication {
	return utils.NewRadixApplicationBuilder().WithAppName(appName).WithEnvironment(environment, envBranch).
		WithJobComponents(utils.NewApplicationJobComponentBuilder().WithName(jobComponentName).WithPort("http", port).WithNotifications(notifications)).
		BuildRA()
}
