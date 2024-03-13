package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/equinor/radix-job-scheduler/api"
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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

func main() {
	env := models.NewEnv()
	initLogger(env)

	kubeUtil := getKubeUtil()

	radixDeployJobComponent, err := radix.GetRadixDeployJobComponentByName(context.Background(), kubeUtil.RadixClient(), env.RadixDeploymentNamespace, env.RadixDeploymentName, env.RadixComponentName)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get job specification")
	}

	radixBatchWatcher, err := getRadixBatchWatcher(kubeUtil, radixDeployJobComponent, env)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to inititialize job watcher")
	}
	defer close(radixBatchWatcher.Stop)

	runApiServer(kubeUtil, env)
}

func initLogger(env *models.Env) {
	logLevelStr := env.LogLevel
	if len(logLevelStr) == 0 {
		logLevelStr = zerolog.LevelInfoValue
	}

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly})
	zerolog.DefaultContextLogger = &log.Logger
}

func runApiServer(kubeUtil *kube.Kube, env *models.Env) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fs := initializeFlagSet()
	var (
		port = fs.StringP("port", "p", env.RadixPort, "Port where API will be served")
	)
	parseFlagsFromArgs(fs)

	go func() {
		log.Info().Msgf("Radix job API is serving on port %s", *port)
		srv := &http.Server{
			Addr:        fmt.Sprintf(":%s", *port),
			Handler:     router.NewServer(env, getControllers(kubeUtil, env)...),
			BaseContext: func(_ net.Listener) context.Context { return ctx },
		}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("Radix job API server stopped")
		}

	}()

	<-ctx.Done()
}

func getRadixBatchWatcher(kubeUtil *kube.Kube, radixDeployJobComponent *radixv1.RadixDeployJobComponent, env *models.Env) (*notifications.Watcher, error) {
	notifier := notifications.NewWebhookNotifier(radixDeployJobComponent)
	log.Info().Msgf("Created notifier: %s", notifier.String())
	if !notifier.Enabled() {
		log.Info().Msg("Notifiers are not enabled, RadixBatch event and changes watcher is skipped.")
		return notifications.NullRadixBatchWatcher(), nil
	}

	return notifications.NewRadixBatchWatcher(kubeUtil.RadixClient(), env.RadixDeploymentNamespace, notifier)
}

func getKubeUtil() *kube.Kube {
	kubeClient, radixClient, _, secretProviderClient, _ := utils.GetKubernetesClient()
	kubeUtil, _ := kube.New(kubeClient, radixClient, secretProviderClient)
	return kubeUtil
}

func getControllers(kubeUtil *kube.Kube, env *models.Env) []api.Controller {
	return []api.Controller{
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
		log.Error().Err(err).Msg("Failed to parse flags")
		fs.Usage()
		os.Exit(2)
	}
}
