package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	batchApi "github.com/equinor/radix-job-scheduler/api/v1/batches"
	batchControllers "github.com/equinor/radix-job-scheduler/api/v1/controllers/batches"
	jobControllers "github.com/equinor/radix-job-scheduler/api/v1/controllers/jobs"
	jobApi "github.com/equinor/radix-job-scheduler/api/v1/jobs"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/notifications"
	"github.com/equinor/radix-job-scheduler/router"
	_ "github.com/equinor/radix-job-scheduler/swaggerui"
	"github.com/equinor/radix-job-scheduler/utils/radix"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/gorilla/handlers"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

func main() {
	env := models.NewEnv()
	kubeUtil := getKubeUtil()

	radixDeployJobComponent, err := radix.GetRadixDeployJobComponentByName(context.Background(), kubeUtil.RadixClient(), env.RadixDeploymentNamespace, env.RadixDeploymentName, env.RadixComponentName)
	if err != nil {
		log.Fatalln(err)
		return
	}

	// TODO: delete me, just development
	s := "http://localhost:8030"
	radixDeployJobComponent.Notifications.Webhook = &s

	radixBatchWatcher, err := getRadixBatchWatcher(kubeUtil, radixDeployJobComponent, env)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer close(radixBatchWatcher.Stop)

	runApiServer(kubeUtil, env)
}

func runApiServer(kubeUtil *kube.Kube, env *models.Env) {
	fs := initializeFlagSet()
	var (
		port = fs.StringP("port", "p", env.RadixPort, "Port where API will be served")
	)
	log.Debugf("Port: %s\n", *port)
	parseFlagsFromArgs(fs)

	errsChan := make(chan error)
	go func() {
		log.Infof("Radix job scheduler API is serving on port %s", *port)
		err := http.ListenAndServe(fmt.Sprintf(":%s", *port), handlers.CombinedLoggingHandler(os.Stdout, router.NewServer(env, getControllers(kubeUtil, env)...)))
		errsChan <- err
	}()

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)

	select {
	case <-sigTerm:
	case err := <-errsChan:
		if err != nil {
			log.Fatalf("Radix job scheduler API server crashed: %v", err)
		}
	}
}

func getRadixBatchWatcher(kubeUtil *kube.Kube, radixDeployJobComponent *radixv1.RadixDeployJobComponent, env *models.Env) (*notifications.Watcher, error) {
	notifier, err := notifications.NewWebhookNotifier(radixDeployJobComponent.Notifications)
	if err != nil {
		return notifications.NullRadixBatchWatcher(), err
	}

	log.Infof("Created notifier: %s", notifier.String())
	if !notifier.Enabled() {
		log.Infoln("Notifiers are not enabled, RadixBatch event and changes watcher is skipped.")
		return notifications.NullRadixBatchWatcher(), nil
	}

	return notifications.NewRadixBatchWatcher(kubeUtil.RadixClient(), env.RadixDeploymentNamespace, notifier)
}

func getKubeUtil() *kube.Kube {
	kubeClient, radixClient, _, secretProviderClient := utils.GetKubernetesClient()
	kubeUtil, _ := kube.New(kubeClient, radixClient, secretProviderClient)
	return kubeUtil
}

func getControllers(kubeUtil *kube.Kube, env *models.Env) []models.Controller {
	return []models.Controller{
		jobControllers.New(jobApi.New(kubeUtil, env)),
		batchControllers.New(batchApi.New(kubeUtil, env)),
	}
}

func initializeFlagSet() *pflag.FlagSet {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "DESCRIPTION\n")
		fmt.Fprint(os.Stderr, "Radix job scheduler API server.\n")
		fmt.Fprint(os.Stderr, "\n")
		fmt.Fprint(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}
	return fs
}

func parseFlagsFromArgs(fs *pflag.FlagSet) {
	err := fs.Parse(os.Args[1:])
	switch {
	case err == pflag.ErrHelp:
		os.Exit(0)
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err.Error())
		fs.Usage()
		os.Exit(2)
	}
}
