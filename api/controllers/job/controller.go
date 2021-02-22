package job

import (
	"context"
	"fmt"
	"net/http"

	jh "github.com/equinor/radix-job-scheduler/api/handlers/job"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/google/martian/log"
	"github.com/gorilla/mux"
)

const jobNameParam = "name"

type jobController struct {
	jobHandler jh.JobHandler
}

// New create a new job controller
func New(jobHandler jh.JobHandler) models.Controller {
	return &jobController{
		jobHandler: jobHandler,
	}
}

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

func (controller *jobController) GetJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := controller.jobHandler.GetJobs(context.Background())
	if err != nil {
		log.Errorf("failed: %v", err)
		utils.ErrorResult(w, r, http.StatusInternalServerError)
		return
	}

	utils.JSONResult(w, r, jobs)
}

func (controller *jobController) GetJob(w http.ResponseWriter, r *http.Request) {
	jobName := mux.Vars(r)[jobNameParam]
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("job %s returned", jobName)))
}

func (controller *jobController) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobName := mux.Vars(r)[jobNameParam]
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("job %s deleted", jobName)))
}
