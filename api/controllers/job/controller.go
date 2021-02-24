package job

import (
	"context"
	"fmt"
	"net/http"

	jh "github.com/equinor/radix-job-scheduler/api/handlers/job"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const jobNameParam = "name"

type jobController struct {
	jobHandler jh.Handler
}

// New create a new job controller
func New(jobHandler jh.Handler) models.Controller {
	return &jobController{
		jobHandler: jobHandler,
	}
}

// GetRoutes List the supported routes of this controller
func (controller *jobController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        "/jobs",
			Method:      http.MethodPost,
			HandlerFunc: controller.CreateJob,
		},
		models.Route{
			Path:        "/jobs",
			Method:      http.MethodGet,
			HandlerFunc: controller.GetJobs,
		},
		models.Route{
			Path:        fmt.Sprintf("/jobs/{%s}", jobNameParam),
			Method:      http.MethodGet,
			HandlerFunc: controller.GetJob,
		},
		models.Route{
			Path:        fmt.Sprintf("/jobs/{%s}", jobNameParam),
			Method:      http.MethodDelete,
			HandlerFunc: controller.DeleteJob,
		},
	}
	return routes
}

func (controller *jobController) CreateJob(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("job created"))
}

// swagger:operation GET /jobs/ job getJobs
// ---
// summary: Gets jobs
// parameters:
// responses:
//   "200":
//     description: "Successful get jobs"
//     schema:
//        "$ref": "#/definitions/Job"
//   "401":
//     description: "Unauthorized"
//   "404":
//     description: "Not found"
func (controller *jobController) GetJobs(w http.ResponseWriter, r *http.Request) {
	log.Debug("Get job list")
	jobs, err := controller.jobHandler.GetJobs(context.Background())
	if err != nil {
		log.Errorf("failed: %v", err)
		utils.ErrorResponse(w, r, http.StatusInternalServerError)
		return
	}
	log.Debugf("Found %d jobs", len(jobs))
	utils.JSONResponse(w, r, jobs)
}

func (controller *jobController) GetJob(w http.ResponseWriter, r *http.Request) {
	jobName := mux.Vars(r)[jobNameParam]
	log.Debugf("Get job %s", jobName)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("job %s returned", jobName)))
}

func (controller *jobController) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobName := mux.Vars(r)[jobNameParam]
	log.Debugf("Delete job %s", jobName)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("job %s deleted", jobName)))
}
