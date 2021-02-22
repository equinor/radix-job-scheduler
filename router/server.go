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
func NewServer(controllers ...models.Controller) http.Handler {
	router := mux.NewRouter().StrictSlash(true)
	initializeAPIServer(router, controllers)

	serveMux := http.NewServeMux()
	serveMux.Handle(apiVersionRoute+"/", router)

	recovery := negroni.NewRecovery()
	recovery.PrintStack = false

	n := negroni.New(recovery)
	n.UseHandler(serveMux)
	return n
}

func initializeAPIServer(router *mux.Router, controllers []models.Controller) {
	for _, controller := range controllers {
		for _, route := range controller.GetRoutes() {
			addHandlerRoute(router, route)
		}
	}
}

func addHandlerRoute(router *mux.Router, route models.Route) {
	path := apiVersionRoute + route.Path
	router.HandleFunc(path,
		utils.NewRadixMiddleware(path, route.Method, route.HandlerFunc).Handle).Methods(route.Method)
}
