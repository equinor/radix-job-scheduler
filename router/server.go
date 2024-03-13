package router

import (
	"net/http"

	commongin "github.com/equinor/radix-common/pkg/gin"
	"github.com/equinor/radix-job-scheduler/api"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/swaggerui"
	"github.com/gin-gonic/gin"
)

const (
	apiVersionRoute = "/api/v1"
	swaggerUIPath   = "/swaggerui"
)

// NewServer creates a new Radix job scheduler REST service
func NewServer(env *models.Env, controllers ...api.Controller) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.RemoveExtraSlash = true
	engine.Use(commongin.ZerologRequestLogger(), gin.Recovery())

	if env.UseSwagger {
		initializeSwaggerUI(engine)
	}

	v1Router := engine.Group(apiVersionRoute)
	{
		initializeAPIServer(v1Router, controllers)
	}

	return engine
}

func initializeSwaggerUI(engine *gin.Engine) {
	swaggerFsHandler := http.FS(swaggerui.FS())
	engine.StaticFS(swaggerUIPath, swaggerFsHandler)
}

func initializeAPIServer(router gin.IRoutes, controllers []api.Controller) {
	for _, controller := range controllers {
		for _, route := range controller.GetRoutes() {
			addHandlerRoute(router, route)
		}
	}
}

func addHandlerRoute(router gin.IRoutes, route api.Route) {
	router.Handle(route.Method, route.Path, route.Handler)
}
