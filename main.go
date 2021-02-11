package main

import (
	"net/http"

	jc "github.com/equinor/radix-job-scheduler/api/controllers/job"
	jh "github.com/equinor/radix-job-scheduler/api/handlers/job"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/router"
	log "github.com/sirupsen/logrus"
)

func main() {
	errs := make(chan error)

	go startHTTPListener(":3003", errs)

	err := <-errs
	if err != nil {
		log.Fatalf("Radix cost allocation api server crashed: %v", err)
	}
}

func startHTTPListener(address string, errs chan error) {
	log.Infof("listening for requests on %s", address)
	kubeUtil := models.NewKubeUtil()
	jobController := jc.New(jh.New(kubeUtil))
	router := router.NewServer(kubeUtil, jobController)
	errs <- http.ListenAndServe(address, router)
}
