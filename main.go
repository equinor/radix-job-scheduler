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
	radixinformers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/cache"
)

const informerResyncPeriod = 0

func main() {
	ctx := context.Background()
	env := models.NewEnv()
	initLogger(env)

	kubeUtil := getKubeUtil(ctx)

	radixDeployJobComponent, err := radix.GetRadixDeployJobComponentByName(ctx, kubeUtil.RadixClient(), env.RadixDeploymentNamespace, env.RadixDeploymentName, env.RadixComponentName)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get job specification")
	}

	radixBatchWatcher, err := getRadixBatchWatcher(kubeUtil, radixDeployJobComponent, env)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to inititialize job watcher")
	}
	defer close(radixBatchWatcher.Stop)

	runApiServer(ctx, kubeUtil, env, radixDeployJobComponent)
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
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.DefaultContextLogger = &log.Logger
}

func runApiServer(ctx context.Context, kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fs := initializeFlagSet()
	port := fs.StringP("port", "p", env.RadixPort, "Port where API will be served")
	parseFlagsFromArgs(fs)

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%s", *port),
		Handler:     router.NewServer(env, getControllers(ctx, kubeUtil, env, radixDeployJobComponent)...),
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	go func() {
		log.Info().Msgf("Radix job API is serving on port %s", *port)
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

func getRadixBatchWatcher(kubeUtil *kube.Kube, radixDeployJobComponent *radixv1.RadixDeployJobComponent, env *models.Env) (*notifications.Watcher, error) {
	notifier := notifications.NewWebhookNotifier(radixDeployJobComponent)
	log.Info().Msgf("Created notifier: %s", notifier.String())
	if !notifier.Enabled() {
		log.Info().Msg("Notifiers are not enabled, RadixBatch event and changes watcher is skipped.")
		return notifications.NullRadixBatchWatcher(), nil
	}

	return notifications.NewRadixBatchWatcher(kubeUtil.RadixClient(), env.RadixDeploymentNamespace, notifier)
}

func getKubeUtil(ctx context.Context) *kube.Kube {
	kubeClient, radixClient, kedaClient, _, secretProviderClient, _ := utils.GetKubernetesClient(ctx)
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretProviderClient)
	return kubeUtil
}

func getControllers(ctx context.Context, kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []api.Controller {
	jobHandler := jobApi.New(kubeUtil, env, radixDeployJobComponent)
	batchHandler := batchApi.New(kubeUtil, env, radixDeployJobComponent)
	setupEventHandlers(ctx, kubeUtil, batchHandler, jobHandler)
	return []api.Controller{
		jobControllers.New(jobHandler),
		batchControllers.New(batchHandler),
	}
}

func setupEventHandlers(ctx context.Context, kubeUtil *kube.Kube, batchHandler batchApi.BatchHandler, jobHandler jobApi.JobHandler) {
	log.Info().Msg("Setting up event handlers")
	batchInformerFactory := radixinformers.NewSharedInformerFactory(kubeUtil.RadixClient(), informerResyncPeriod)
	batchInformer := batchInformerFactory.Radix().V1().RadixBatches().Informer()
	if _, err := batchInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			if !batchInformer.HasSynced() {
				return
			}
			radixBatch, converted := cur.(*radixv1.RadixBatch)
			if !converted {
				log.Error().Msg("Failed to cast RadixBatch object")
				return
			}
			if radixBatch.Status.Condition.Type != "" {
				return // skip existing batch added to the cache
			}
			log.Debug().Msgf("RadixBatch object added: %s", radixBatch.GetName())
			if radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeBatch) {
				batchHandler.CleanupJobHistory(ctx)
			} else {
				jobHandler.CleanupJobHistory(ctx)
			}
		},
	}); err != nil {
		panic(err)
	}

	batchInformerFactory.Start(ctx.Done())
	log.Info().Msg("Waiting for Radix objects caches to sync")
	batchInformerFactory.WaitForCacheSync(ctx.Done())
	log.Info().Msg("Completed syncing informer caches")
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
