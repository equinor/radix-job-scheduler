package batch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/equinor/radix-job-scheduler/api/controllers"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	api "github.com/equinor/radix-job-scheduler/api/v1/batches"
	"github.com/equinor/radix-job-scheduler/models"
	schedulerModels "github.com/equinor/radix-job-scheduler/models/common"
	"github.com/equinor/radix-job-scheduler/utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const (
	batchNameParam = "batchName"
	jobNameParam   = "jobName"
)

type batchController struct {
	*controllers.ControllerBase
	handler api.BatchHandler
}

// New create a new batch controller
func New(handler api.BatchHandler) models.Controller {
	return &batchController{
		handler: handler,
	}
}

// GetRoutes List the supported routes of this controller
func (controller *batchController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        "/batches",
			Method:      http.MethodPost,
			HandlerFunc: controller.CreateBatch,
		},
		models.Route{
			Path:        "/batches",
			Method:      http.MethodGet,
			HandlerFunc: controller.GetBatches,
		},
		models.Route{
			Path:        fmt.Sprintf("/batches/{%s}", batchNameParam),
			Method:      http.MethodGet,
			HandlerFunc: controller.GetBatch,
		},
		models.Route{
			Path:        fmt.Sprintf("/batches/{%s}/jobs/{%s}", batchNameParam, jobNameParam),
			Method:      http.MethodGet,
			HandlerFunc: controller.GetBatchJob,
		},
		models.Route{
			Path:        fmt.Sprintf("/batches/{%s}", batchNameParam),
			Method:      http.MethodDelete,
			HandlerFunc: controller.DeleteBatch,
		},
		models.Route{
			Path:        fmt.Sprintf("/batches/{%s}/stop", batchNameParam),
			Method:      http.MethodPost,
			HandlerFunc: controller.StopBatch,
		},
		models.Route{
			Path:        fmt.Sprintf("/batches/{%s}/jobs/{%s}/stop", batchNameParam, jobNameParam),
			Method:      http.MethodPost,
			HandlerFunc: controller.StopBatchJob,
		},
	}
	return routes
}

// swagger:operation POST /batches Batch createBatch
// ---
// summary: Create batch
// parameters:
// - name: batchCreation
//   in: body
//   description: Batch to create
//   required: true
//   schema:
//       "$ref": "#/definitions/BatchScheduleDescription"
// responses:
//   "200":
//     description: "Successful create batch"
//     schema:
//        "$ref": "#/definitions/BatchStatus"
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
func (controller *batchController) CreateBatch(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Create Batch. Request content length %d", r.ContentLength)
	var batchScheduleDescription schedulerModels.BatchScheduleDescription

	log.Debugf("Read the request body")
	if body, _ := io.ReadAll(r.Body); len(body) > 0 {
		log.Debugf("Read %d bytes", len(body))
		if err := json.Unmarshal(body, &batchScheduleDescription); err != nil {
			log.Errorf("failed to unmarshal batchScheduleDescription: %v", err)
			controller.HandleError(w, apiErrors.NewInvalid("BatchScheduleDescription"))
			return
		}
	}

	batchState, err := controller.handler.CreateBatch(r.Context(), &batchScheduleDescription)
	if err != nil {
		controller.HandleError(w, err)
		return
	}
	err = controller.handler.MaintainHistoryLimit(r.Context())
	if err != nil {
		log.Warnf("failed to maintain batch history: %v", err)
	}

	utils.JSONResponse(w, &batchState)
}

// swagger:operation GET /batches/ Batch getBatches
// ---
// summary: Gets batches
// parameters:
// responses:
//   "200":
//     description: "Successful get batches"
//     schema:
//        type: "array"
//        items:
//           "$ref": "#/definitions/BatchStatus"
//   "500":
//     description: "Internal server error"
//     schema:
//        "$ref": "#/definitions/Status"
func (controller *batchController) GetBatches(w http.ResponseWriter, r *http.Request) {
	log.Debug("Get batch list")
	batches, err := controller.handler.GetBatches(r.Context())
	if err != nil {
		controller.HandleError(w, err)
		return
	}
	log.Debugf("Found %d batches", len(batches))
	utils.JSONResponse(w, batches)
}

// swagger:operation GET /batches/{batchName} Batch getBatch
// ---
// summary: Gets batch
// parameters:
// - name: batchName
//   in: path
//   description: Name of batch
//   type: string
//   required: true
// responses:
//   "200":
//     description: "Successful get batch"
//     schema:
//        "$ref": "#/definitions/BatchStatus"
//   "404":
//     description: "Not found"
//     schema:
//        "$ref": "#/definitions/Status"
//   "500":
//     description: "Internal server error"
//     schema:
//        "$ref": "#/definitions/Status"
func (controller *batchController) GetBatch(w http.ResponseWriter, r *http.Request) {
	batchName := mux.Vars(r)[batchNameParam]
	log.Debugf("Get batch %s", batchName)
	batch, err := controller.handler.GetBatch(r.Context(), batchName)
	if err != nil {
		controller.HandleError(w, err)
		return
	}
	utils.JSONResponse(w, batch)
}

// swagger:operation GET /batches/{batchName}/jobs/{jobName} Batch getBatchJob
// ---
// summary: Gets batch job
// parameters:
// - name: batchName
//   in: path
//   description: Name of batch
//   type: string
//   required: true
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
func (controller *batchController) GetBatchJob(w http.ResponseWriter, r *http.Request) {
	batchName := mux.Vars(r)[batchNameParam]
	jobName := mux.Vars(r)[jobNameParam]
	log.Debugf("Get job %s from the batch %s", jobName, batchName)
	job, err := controller.handler.GetBatchJob(r.Context(), batchName, jobName)
	if err != nil {
		controller.HandleError(w, err)
		return
	}
	utils.JSONResponse(w, job)
}

// swagger:operation DELETE /batches/{batchName} Batch deleteBatch
// ---
// summary: Delete batch
// parameters:
// - name: batchName
//   in: path
//   description: Name of batch
//   type: string
//   required: true
// responses:
//   "200":
//     description: "Successful delete batch"
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
func (controller *batchController) DeleteBatch(w http.ResponseWriter, r *http.Request) {
	batchName := mux.Vars(r)[batchNameParam]
	log.Debugf("Delete batch %s", batchName)
	err := controller.handler.DeleteBatch(r.Context(), batchName)
	if err != nil {
		controller.HandleError(w, err)
		return
	}

	status := schedulerModels.Status{
		Status:  schedulerModels.StatusSuccess,
		Code:    http.StatusOK,
		Message: fmt.Sprintf("batch %s successfully deleted", batchName),
	}
	utils.StatusResponse(w, &status)
}

// swagger:operation POST /batches/{batchName}/stop Batch stopBatch
// ---
// summary: Stop batch
// parameters:
// - name: batchName
//   in: path
//   description: Name of batch
//   type: string
//   required: true
// responses:
//   "200":
//     description: "Successful stop batch"
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
func (controller *batchController) StopBatch(w http.ResponseWriter, r *http.Request) {
	batchName := mux.Vars(r)[batchNameParam]
	err := controller.handler.StopBatch(r.Context(), batchName)
	if err != nil {
		controller.HandleError(w, err)
		return
	}

	status := schedulerModels.Status{
		Status:  schedulerModels.StatusSuccess,
		Code:    http.StatusOK,
		Message: fmt.Sprintf("batch %s successfully stopped", batchName),
	}
	utils.StatusResponse(w, &status)
}

// swagger:operation POST /batches/{batchName}/jobs/{jobName}/stop Batch stopBatchJob
// ---
// summary: Stop batch job
// parameters:
// - name: batchName
//   in: path
//   description: Name of batch
//   type: string
//   required: true
// - name: jobName
//   in: path
//   description: Name of job
//   type: string
//   required: true
// responses:
//   "200":
//     description: "Successful stop batch job"
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
func (controller *batchController) StopBatchJob(w http.ResponseWriter, r *http.Request) {
	batchName := mux.Vars(r)[batchNameParam]
	jobName := mux.Vars(r)[jobNameParam]
	log.Debugf("Stop the job %s in the batch %s ", jobName, batchName)
	err := controller.handler.StopBatchJob(r.Context(), batchName, jobName)
	if err != nil {
		controller.HandleError(w, err)
		return
	}

	status := schedulerModels.Status{
		Status:  schedulerModels.StatusSuccess,
		Code:    http.StatusOK,
		Message: fmt.Sprintf("job %s in the batch %s successfully stopped", jobName, batchName),
	}
	utils.StatusResponse(w, &status)
}
