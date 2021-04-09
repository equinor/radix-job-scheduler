package job

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/equinor/radix-job-scheduler/api/controllers/testutils"
	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
	jobHandlers "github.com/equinor/radix-job-scheduler/api/handlers/job"
	jobHandlersTest "github.com/equinor/radix-job-scheduler/api/handlers/job/test"
	"github.com/equinor/radix-job-scheduler/api/utils"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func setupTest(jobHandler jobHandlers.Handler) *testutils.ControllerTestUtils {
	jobController := jobController{jobHandler: jobHandler}
	controllerTestUtils := testutils.New(&jobController)
	return &controllerTestUtils
}

func TestGetJobs(t *testing.T) {
	t.Run("Get jobs - success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := jobHandlersTest.NewMockHandler(ctrl)
		jobState := models.JobStatus{
			Name:    "jobname",
			Started: utils.FormatTimestamp(time.Now()),
			Ended:   utils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		jobHandler.
			EXPECT().
			GetJobs().
			Return([]models.JobStatus{jobState}, nil).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, "api/v1/jobs")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJobs []models.JobStatus
			testutils.GetResponseBody(response, &returnedJobs)
			assert.Len(t, returnedJobs, 1)
			assert.Equal(t, jobState.Name, returnedJobs[0].Name)
			assert.Equal(t, jobState.Started, returnedJobs[0].Started)
			assert.Equal(t, jobState.Ended, returnedJobs[0].Ended)
			assert.Equal(t, jobState.Status, returnedJobs[0].Status)
		}
	})

	t.Run("Get jobs - status code 500", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := jobHandlersTest.NewMockHandler(ctrl)
		jobHandler.
			EXPECT().
			GetJobs().
			Return(nil, errors.New("unhandled error")).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, "api/v1/jobs")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			testutils.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestGetJob(t *testing.T) {
	t.Run("Get job - success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName := "jobname"
		jobHandler := jobHandlersTest.NewMockHandler(ctrl)
		jobState := models.JobStatus{
			Name:    jobName,
			Started: utils.FormatTimestamp(time.Now()),
			Ended:   utils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		jobHandler.
			EXPECT().
			GetJob(jobName).
			Return(&jobState, nil).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s", jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJob models.JobStatus
			testutils.GetResponseBody(response, &returnedJob)
			assert.Equal(t, jobState.Name, returnedJob.Name)
			assert.Equal(t, jobState.Started, returnedJob.Started)
			assert.Equal(t, jobState.Ended, returnedJob.Ended)
			assert.Equal(t, jobState.Status, returnedJob.Status)
		}
	})

	t.Run("Get job - not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := jobHandlersTest.NewMockHandler(ctrl)
		jobHandler.
			EXPECT().
			GetJob(gomock.Any()).
			Return(nil, jobErrors.NewNotFound("job", "jobname")).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s", "anyjob"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			testutils.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
		}
	})

	t.Run("Get job - internal error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := jobHandlersTest.NewMockHandler(ctrl)
		jobHandler.
			EXPECT().
			GetJob(gomock.Any()).
			Return(nil, errors.New("unhandled error")).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s", "anyjob"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			testutils.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestCreateJob(t *testing.T) {
	t.Run("create job with valid payload body - successful", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobScheduleDescription := models.JobScheduleDescription{
			Payload: "a_payload",
		}
		createdJob := models.JobStatus{
			Name:    "newjob",
			Started: utils.FormatTimestamp(time.Now()),
			Ended:   utils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		jobHandler := jobHandlersTest.NewMockHandler(ctrl)
		jobHandler.
			EXPECT().
			CreateJob(&jobScheduleDescription).
			Return(&createdJob, nil).
			Times(1)
		jobHandler.
			EXPECT().
			MaintainHistoryLimit().
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(http.MethodPost, "/api/v1/jobs", jobScheduleDescription)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJob models.JobStatus
			testutils.GetResponseBody(response, &returnedJob)
			assert.Equal(t, createdJob.Name, returnedJob.Name)
			assert.Equal(t, createdJob.Started, returnedJob.Started)
			assert.Equal(t, createdJob.Ended, returnedJob.Ended)
			assert.Equal(t, createdJob.Status, returnedJob.Status)
		}
	})

	t.Run("create job with valid payload body - error from MaintainHistoryLimit should not fail request", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobScheduleDescription := models.JobScheduleDescription{
			Payload: "a_payload",
		}
		createdJob := models.JobStatus{
			Name:    "newjob",
			Started: utils.FormatTimestamp(time.Now()),
			Ended:   utils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		jobHandler := jobHandlersTest.NewMockHandler(ctrl)
		jobHandler.
			EXPECT().
			CreateJob(&jobScheduleDescription).
			Return(&createdJob, nil).
			Times(1)
		jobHandler.
			EXPECT().
			MaintainHistoryLimit().
			Return(errors.New("an error")).
			Times(1)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(http.MethodPost, "/api/v1/jobs", jobScheduleDescription)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJob models.JobStatus
			testutils.GetResponseBody(response, &returnedJob)
			assert.Equal(t, createdJob.Name, returnedJob.Name)
			assert.Equal(t, createdJob.Started, returnedJob.Started)
			assert.Equal(t, createdJob.Ended, returnedJob.Ended)
			assert.Equal(t, createdJob.Status, returnedJob.Status)
		}
	})

	t.Run("create job with invalid request body - unprocessable", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		jobHandler := jobHandlersTest.NewMockHandler(ctrl)
		jobHandler.
			EXPECT().
			CreateJob(gomock.Any()).
			Times(0)
		jobHandler.
			EXPECT().
			MaintainHistoryLimit().
			Times(0)
		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(http.MethodPost, "/api/v1/jobs", struct{ Payload interface{} }{Payload: struct{}{}})
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
			var returnedStatus models.Status
			testutils.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusUnprocessableEntity, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonInvalid, returnedStatus.Reason)
		}
	})
}
