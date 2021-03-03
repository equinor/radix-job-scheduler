package job

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	jh "github.com/equinor/radix-job-scheduler/api/handlers/job"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const jobNameParam = "jobName"

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

// swagger:operation POST /jobs/{jobName} job createJob
// ---
// summary: Gets jobs
// parameters:
// - name: jobCreation
//   in: body
//   description: Job to create
//   required: true
//   schema:
//       "$ref": "#/definitions/ApplicationRegistration"
// responses:
//   "200":
//     description: "Successful create job"
//     schema:
//        "$ref": "#/definitions/JobStatus"
//   "401":
//     description: "Unauthorized"
//   "404":
//     description: "Not found"
func (controller *jobController) CreateJob(w http.ResponseWriter, r *http.Request) {
	var application models.JobStatus
	if err := json.NewDecoder(r.Body).Decode(&application); err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	// Need in cluster Radix client in order to validate registration using sufficient priviledges
	handler := Init(accounts)
	appRegistration, err := handler.RegisterApplication(application)
	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &appRegistration)
}

// swagger:operation GET /jobs/ job getJobs
// ---
// summary: Gets jobs
// parameters:
// responses:
//   "200":
//     description: "Successful get jobs"
//     schema:
//        "$ref": "#/definitions/JobStatus"
//   "401":
//     description: "Unauthorized"
//   "404":
//     description: "Not found"
func (controller *jobController) GetJobs(w http.ResponseWriter, r *http.Request) {
	log.Debug("Get job list")
	jobs, err := controller.jobHandler.GetJobs(context.Background())
	if err != nil {
		log.Errorf("failed: %v", err)
		utils.WriteResponse(w, http.StatusInternalServerError)
		return
	}
	log.Debugf("Found %d jobs", len(*jobs))
	utils.JSONResponse(w, r, jobs)
}

// swagger:operation GET /jobs/{jobName} job getJobs
// ---
// summary: Gets jobs
// parameters:
// - name: jobName
//   in: path
//   description: Name of job
//   type: string
//   required: true
// responses:
//   "200":
//     description: "Successful get job"
//     schema:
//        "$ref": "#/definitions/JobStatus"
//   "401":
//     description: "Unauthorized"
//   "404":
//     description: "Not found"
func (controller *jobController) GetJob(w http.ResponseWriter, r *http.Request) {
	jobName := mux.Vars(r)[jobNameParam]
	log.Debugf("Get job %s", jobName)
	job, err := controller.jobHandler.GetJob(context.Background(), jobName)
	if err != nil {
		log.Errorf("failed: %v", err)
		utils.WriteResponse(w, http.StatusInternalServerError)
		return
	}
	utils.JSONResponse(w, r, job)
}

// swagger:operation DELETE /jobs/{jobName} job deleteJob
// ---
// summary: Delete job
// parameters:
// - name: jobName
//   in: path
//   description: Name of job
//   type: string
//   required: true
// responses:
//   "200":
//     description: "Successful delete job"
//     schema:
//        "$ref": "#/definitions/JobStatus"
//   "401":
//     description: "Unauthorized"
//   "404":
//     description: "Not found"
func (controller *jobController) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobName := mux.Vars(r)[jobNameParam]
	log.Debugf("Delete job %s", jobName)
	err := controller.jobHandler.DeleteJob(context.Background(), jobName)
	if err != nil {
		log.Errorf("failed: %v", err)
		utils.WriteResponse(w, http.StatusInternalServerError)
		return
	}
	utils.WriteResponse(w, http.StatusOK, fmt.Sprintf("job %s deleted", jobName))
}
