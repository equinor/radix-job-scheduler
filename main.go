package main

import (
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

	radixApplication, radixDeployJobComponent, err := getRadixObjects(kubeUtil, env)
	if err != nil {
		log.Fatalln(err)
		return
	}

	radixBatchWatcher, err := getRadixBatchWatcher(kubeUtil, radixApplication, radixDeployJobComponent, env)
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

func getRadixBatchWatcher(kubeUtil *kube.Kube, radixApplication *radixv1.RadixApplication, radixDeployJobComponent *radixv1.RadixDeployJobComponent, env *models.Env) (*radix.Watcher, error) {
	notifier, err := radix.NewWebhookNotifier(radixApplication, radixDeployJobComponent.Notifications, env)
	if err != nil {
		return radix.NullRadixBatchWatcher(), err
	}

	log.Infof("Created notifier: %s", notifier.String())
	if !notifier.Enabled() {
		log.Infoln("Notifiers are not enabled, RadixBatch event and changes watcher is skipped.")
		return radix.NullRadixBatchWatcher(), nil
	}

	return radix.NewRadixBatchWatcher(kubeUtil.RadixClient(), env.RadixDeploymentNamespace, notifier)
}

func getRadixObjects(kubeUtil *kube.Kube, env *models.Env) (*radixv1.RadixApplication, *radixv1.RadixDeployJobComponent, error) {
	radixApplication, err := radix.GetRadixApplicationByName(kubeUtil.RadixClient(), env.RadixAppName)
	if err != nil {
		return nil, nil, err
	}
	radixDeployJobComponent, err := radix.GetRadixDeployJobComponentByName(kubeUtil.RadixClient(), env.RadixDeploymentNamespace, env.RadixDeploymentName, env.RadixComponentName)
	if err != nil {
		return nil, nil, err
	}
	return radixApplication, radixDeployJobComponent, err
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
