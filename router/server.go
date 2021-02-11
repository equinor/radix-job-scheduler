package router

import (
	"net/http"

	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

const apiVersionRoute = "/api/v1"

// NewServer creates a new Radix job scheduler REST service
func NewServer(kubeUtil models.KubeUtil, controllers ...models.Controller) http.Handler {
	router := mux.NewRouter().StrictSlash(true)
	initializeAPIServer(kubeUtil, router, controllers)

	serveMux := http.NewServeMux()
	serveMux.Handle(apiVersionRoute+"/", router)

	recovery := negroni.NewRecovery()
	recovery.PrintStack = false

	n := negroni.New(recovery)
	n.UseHandler(serveMux)
	return n
}

func initializeAPIServer(kubeUtil models.KubeUtil, router *mux.Router, controllers []models.Controller) {
	for _, controller := range controllers {
		for _, route := range controller.GetRoutes() {
			addHandlerRoute(kubeUtil, router, route)
		}
	}
}

func addHandlerRoute(kubeUtil models.KubeUtil, router *mux.Router, route models.Route) {
	path := apiVersionRoute + route.Path
	router.HandleFunc(path,
		utils.NewRadixMiddleware(path, route.Method, route.HandlerFunc, kubeUtil).Handle).Methods(route.Method)
}
