package main

import (
	"fmt"
	cj "github.com/equinor/radix-job-scheduler/api/controllers/job"
	ch "github.com/equinor/radix-job-scheduler/api/handlers/job"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/router"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"net/http"
	"os"
)

func main() {
	env := models.NewEnv()
	fs := initializeFlagSet()

	var (
		port = fs.StringP("port", "p", defaultPort(), "Port where API will be served")
		//useOutClusterClient = fs.Bool("useOutClusterClient", true, "In case of testing on local machine you may want to set this to false")
	)

	log.Debugf("Port: %s\n", *port)
	parseFlagsFromArgs(fs)

	errs := make(chan error)

	go func() {
		log.Infof("Radix job scheduler API is serving on port %s", *port)
		err := http.ListenAndServe(fmt.Sprintf(":%s", *port), router.NewServer(env, getControllers(env)...))
		errs <- err
	}()

	err := <-errs
	if err != nil {
		log.Fatalf("Radix job scheduler API server crashed: %v", err)
	}
}

func getControllers(env *models.Env) []models.Controller {
	kubeUtil := models.NewKubeUtil(env)
	return []models.Controller{
		cj.New(ch.New(kubeUtil)),
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

func defaultPort() string {
	return "8080"
}
