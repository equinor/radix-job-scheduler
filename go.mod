module github.com/equinor/radix-job-scheduler

go 1.13

require (
	github.com/equinor/radix-operator v1.8.4-RC2
	github.com/gorilla/mux v1.8.0
	github.com/rakyll/statik v0.1.6
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/urfave/negroni v1.0.0
	k8s.io/api v0.18.10
	k8s.io/apimachinery v0.18.10
	k8s.io/client-go v12.0.0+incompatible
)

replace (
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.0.0-20190818123050-43acd0e2e93f
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)
