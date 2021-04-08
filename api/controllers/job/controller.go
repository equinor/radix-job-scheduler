package job

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/equinor/radix-job-scheduler/api/controllers"

	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
	jh "github.com/equinor/radix-job-scheduler/api/handlers/job"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const jobNameParam = "jobName"

type jobController struct {
	*controllers.ControllerBase
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

// swagger:operation POST /jobs Job createJob
// ---
// summary: Create job
// parameters:
// - name: jobCreation
//   in: body
//   description: Job to create
//   required: true
//   schema:
//       "$ref": "#/definitions/JobScheduleDescription"
// responses:
//   "200":
//     description: "Successful create job"
//     schema:
//        "$ref": "#/definitions/JobStatus"
//   "400":
//     description: "Bad request"
//     schema:
//        "$ref": "#/definitions/Status"
//   "404":
//     description: "Not found"
//     schema:
//        "$ref": "#/definitions/Status"
//   "422":
//     description: "Invalid data in request"
//     schema:
//        "$ref": "#/definitions/Status"
//   "500":
//     description: "Internal server error"
//     schema:
//        "$ref": "#/definitions/Status"
func (controller *jobController) CreateJob(w http.ResponseWriter, r *http.Request) {
	var jobScheduleDescription models.JobScheduleDescription
	if err := json.NewDecoder(r.Body).Decode(&jobScheduleDescription); err != nil {
		controller.HandleError(w, jobErrors.NewInvalid("payload"))
		return
	}

	jobState, err := controller.jobHandler.CreateJob(&jobScheduleDescription)
	if err != nil {
		controller.HandleError(w, err)
		return
	}
	err = controller.jobHandler.MaintainHistoryLimit()
	if err != nil {
		log.Warnf("failed to maintain job history: %v", err)
	}

	utils.JSONResponse(w, &jobState)
}

// swagger:operation GET /jobs/ Job getJobs
// ---
// summary: Gets jobs
// parameters:
// responses:
//   "200":
//     description: "Successful get jobs"
//     schema:
//        "$ref": "#/definitions/JobStatus"
//   "500":
//     description: "Internal server error"
//     schema:
//        "$ref": "#/definitions/Status"
func (controller *jobController) GetJobs(w http.ResponseWriter, r *http.Request) {
	log.Debug("Get job list")
	jobs, err := controller.jobHandler.GetJobs()
	if err != nil {
		controller.HandleError(w, err)
		return
	}
	log.Debugf("Found %d jobs", len(jobs))
	utils.JSONResponse(w, jobs)
}

// swagger:operation GET /jobs/{jobName} Job getJob
// ---
// summary: Gets job
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
//   "404":
//     description: "Not found"
//     schema:
//        "$ref": "#/definitions/Status"
//   "500":
//     description: "Internal server error"
//     schema:
//        "$ref": "#/definitions/Status"
func (controller *jobController) GetJob(w http.ResponseWriter, r *http.Request) {
	jobName := mux.Vars(r)[jobNameParam]
	log.Debugf("Get job %s", jobName)
	job, err := controller.jobHandler.GetJob(jobName)
	if err != nil {
		controller.HandleError(w, err)
		return
	}
	utils.JSONResponse(w, job)
}

// swagger:operation DELETE /jobs/{jobName} Job deleteJob
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
//        "$ref": "#/definitions/Status"
//   "404":
//     description: "Not found"
//     schema:
//        "$ref": "#/definitions/Status"
//   "500":
//     description: "Internal server error"
//     schema:
//        "$ref": "#/definitions/Status"
func (controller *jobController) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobName := mux.Vars(r)[jobNameParam]
	log.Debugf("Delete job %s", jobName)
	err := controller.jobHandler.DeleteJob(jobName)
	if err != nil {
		controller.HandleError(w, err)
		return
	}

	status := models.Status{
		Status:  "Success",
		Code:    http.StatusOK,
		Message: fmt.Sprintf("job %s successfully deleted", jobName),
	}
	utils.StatusResponse(w, &status)
}
