package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/equinor/radix-job-scheduler/api/controllers"
	batchcontroller "github.com/equinor/radix-job-scheduler/api/controllers/batches"
	jobcontrollers "github.com/equinor/radix-job-scheduler/api/controllers/jobs"
	batchApi "github.com/equinor/radix-job-scheduler/api/v1/batches"
	jobApi "github.com/equinor/radix-job-scheduler/api/v1/jobs"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/notifications"
	"github.com/equinor/radix-job-scheduler/internal/router"
	"github.com/equinor/radix-job-scheduler/internal/watcher"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	_ "github.com/equinor/radix-job-scheduler/swaggerui"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

func main() {
	ctx := context.Background()
	cfg := config.NewConfigFromEnv()
	initLogger(cfg)

	kubeUtil := getKubeUtil(ctx)

	radixDeployJobComponent, err := internal.GetRadixDeployJobComponentByName(ctx, kubeUtil.RadixClient(), cfg.RadixDeploymentNamespace, cfg.RadixDeploymentName, cfg.RadixComponentName)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get job specification")
	}

	jobHistory := batch.NewHistory(kubeUtil, cfg, radixDeployJobComponent)
	notifier := notifications.NewWebhookNotifier(radixDeployJobComponent)
	radixBatchWatcher, err := watcher.NewRadixBatchWatcher(ctx, kubeUtil.RadixClient(), cfg.RadixDeploymentNamespace, jobHistory, notifier)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize job watcher")
	}
	defer radixBatchWatcher.Stop()

	runApiServer(ctx, kubeUtil, cfg, radixDeployJobComponent)
}

func initLogger(cfg *config.Config) {
	logLevelStr := cfg.LogLevel
	if len(logLevelStr) == 0 {
		logLevelStr = zerolog.LevelInfoValue
	}

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.DefaultContextLogger = &log.Logger
}

func runApiServer(ctx context.Context, kubeUtil *kube.Kube, cfg *config.Config, radixDeployJobComponent *radixv1.RadixDeployJobComponent) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fs := initializeFlagSet()
	port := fs.StringP("port", "p", cfg.RadixPort, "Port where API will be served")
	parseFlagsFromArgs(fs)

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%s", *port),
		Handler:     router.NewServer(cfg, getControllers(kubeUtil, cfg, radixDeployJobComponent)...),
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	go func() {
		log.Info().Msgf("Radix job API is serving on port %s, http://localhost:%s/swaggerui", *port, *port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("Radix job API server failed to start")
		}
		log.Info().Msg("Radix job API server stopped")
	}()

	<-ctx.Done()
	err := srv.Shutdown(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("Radix job API failed to stop")
	}
}

func getKubeUtil(ctx context.Context) *kube.Kube {
	kubeClient, radixClient, kedaClient, _, secretProviderClient, _ := utils.GetKubernetesClient(ctx)
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretProviderClient)
	return kubeUtil
}

func getControllers(kubeUtil *kube.Kube, cfg *config.Config, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []controllers.Controller {
	return []controllers.Controller{
		jobcontrollers.New(jobApi.New(kubeUtil, cfg, radixDeployJobComponent)),
		batchcontroller.New(batchApi.New(kubeUtil, cfg, radixDeployJobComponent)),
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
