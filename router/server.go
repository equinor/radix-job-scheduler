package router

import (
	"net/http"

	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/urfave/negroni/v2"
)

const apiVersionRoute = "/api/v1"

// NewServer creates a new Radix job scheduler REST service
func NewServer(env *models.Env, controllers ...models.Controller) http.Handler {
	router := mux.NewRouter().StrictSlash(true)

	if env.UseSwagger {
		initSwagger(router)
	}

	initializeAPIServer(router, controllers)

	serveMux := http.NewServeMux()
	serveMux.Handle(apiVersionRoute+"/", router)

	if env.UseSwagger {
		serveMux.Handle("/swaggerui/", negroni.New(negroni.Wrap(router)))
	}

	recovery := negroni.NewRecovery()
	recovery.PrintStack = false

	n := negroni.New(recovery)
	n.UseHandler(serveMux)
	return n
}

func initSwagger(router *mux.Router) {
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	sh := http.StripPrefix("/swaggerui/", staticServer)
	router.PathPrefix("/swaggerui/").Handler(sh)
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
