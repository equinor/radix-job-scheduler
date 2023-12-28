package router

import (
	"net/http"

	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/swaggerui"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"
)

const (
	apiVersionRoute = "/api/v1"
	swaggerUIPath   = "/swaggerui"
)

// NewServer creates a new Radix job scheduler REST service
func NewServer(env *models.Env, controllers ...models.Controller) http.Handler {
	router := mux.NewRouter().StrictSlash(true)

	if env.UseSwagger {
		initializeSwaggerUI(router)
	}

	initializeAPIServer(router, controllers)

	serveMux := http.NewServeMux()
	serveMux.Handle(apiVersionRoute+"/", router)

	if env.UseSwagger {
		serveMux.Handle(swaggerUIPath+"/", negroni.New(negroni.Wrap(router)))
	}

	recovery := negroni.NewRecovery()
	recovery.PrintStack = false

	n := negroni.New(recovery)
	n.UseHandler(serveMux)
	return n
}

func initializeSwaggerUI(router *mux.Router) {
	swaggerFsHandler := http.FileServer(http.FS(swaggerui.FS()))
	swaggerui := http.StripPrefix(swaggerUIPath, swaggerFsHandler)
	router.PathPrefix(swaggerUIPath).Handler(swaggerui)
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
