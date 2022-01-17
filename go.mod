module github.com/equinor/radix-job-scheduler

go 1.16

require (
	github.com/equinor/radix-common v1.1.6
	github.com/equinor/radix-operator v1.16.13
	github.com/sirupsen/logrus v1.8.1
	k8s.io/api v0.19.9
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v12.0.0+incompatible
)

replace k8s.io/client-go => k8s.io/client-go v0.19.9
