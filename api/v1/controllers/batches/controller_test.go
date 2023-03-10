package batch

import (
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"net/http"
	"testing"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/api/test"
	api "github.com/equinor/radix-job-scheduler/api/v1/batches"
	"github.com/equinor/radix-job-scheduler/api/v1/batches/mock"
	models "github.com/equinor/radix-job-scheduler/models/common"
	modelsV1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func setupTest(handler api.BatchHandler) *test.ControllerTestUtils {
	controller := batchController{handler: handler}
	controllerTestUtils := test.New(&controller)
	return &controllerTestUtils
}

func TestGetBatches(t *testing.T) {
	t.Run("Get batches - success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchState := modelsV1.BatchStatus{
			JobStatus: modelsV1.JobStatus{
				Name:    "batchname",
				Started: commonUtils.FormatTimestamp(time.Now()),
				Ended:   commonUtils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
				Status:  "batchstatus",
			},
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		batchHandler.
			EXPECT().
			GetBatches().
			Return([]modelsV1.BatchStatus{batchState}, nil).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, "api/v1/batches")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatches []modelsV1.BatchStatus
			test.GetResponseBody(response, &returnedBatches)
			assert.Len(t, returnedBatches, 1)
			assert.Equal(t, batchState.JobStatus.Name, returnedBatches[0].Name)
			assert.Equal(t, batchState.JobStatus.Started, returnedBatches[0].Started)
			assert.Equal(t, batchState.JobStatus.Ended, returnedBatches[0].Ended)
			assert.Equal(t, batchState.JobStatus.Status, returnedBatches[0].Status)
		}
	})

	t.Run("Get batches - status code 500", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			GetBatches().
			Return(nil, apiErrors.NewUnknown(fmt.Errorf("unhandled error"))).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, "api/v1/batches")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestGetBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "batchname"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchState := modelsV1.BatchStatus{
			JobStatus: modelsV1.JobStatus{
				Name:    batchName,
				Started: commonUtils.FormatTimestamp(time.Now()),
				Ended:   commonUtils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
				Status:  "batchstatus",
			},
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		batchHandler.
			EXPECT().
			GetBatch(batchName).
			Return(&batchState, nil).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatch modelsV1.BatchStatus
			test.GetResponseBody(response, &returnedBatch)
			assert.Equal(t, batchState.Name, returnedBatch.Name)
			assert.Equal(t, batchState.Started, returnedBatch.Started)
			assert.Equal(t, batchState.Ended, returnedBatch.Ended)
			assert.Equal(t, batchState.Status, returnedBatch.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName, kind := "anybatch", "batch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			GetBatch(gomock.Any()).
			Return(nil, apiErrors.NewNotFound(kind, batchName)).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage(kind, batchName), returnedStatus.Message)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			GetBatch(gomock.Any()).
			Return(nil, errors.New("unhandled error")).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/batches/%s", "anybatch"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestCreateBatch(t *testing.T) {
	t.Run("empty body - successful", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchScheduleDescription := models.BatchScheduleDescription{}
		createdBatch := modelsV1.BatchStatus{
			JobStatus: modelsV1.JobStatus{
				Name:    "newbatch",
				Started: commonUtils.FormatTimestamp(time.Now()),
				Ended:   commonUtils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
				Status:  "batchstatus",
			},
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			CreateBatch(&batchScheduleDescription).
			Return(&createdBatch, nil).
			Times(1)
		batchHandler.
			EXPECT().
			MaintainHistoryLimit().
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(http.MethodPost, "/api/v1/batches", nil)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatch modelsV1.BatchStatus
			test.GetResponseBody(response, &returnedBatch)
			assert.Equal(t, createdBatch.Name, returnedBatch.Name)
			assert.Equal(t, createdBatch.Started, returnedBatch.Started)
			assert.Equal(t, createdBatch.Ended, returnedBatch.Ended)
			assert.Equal(t, createdBatch.Status, returnedBatch.Status)
		}
	})

	t.Run("valid payload body - successful", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchScheduleDescription := models.BatchScheduleDescription{
			JobScheduleDescriptions: []models.JobScheduleDescription{
				{
					Payload: "a_payload",
					RadixJobComponentConfig: models.RadixJobComponentConfig{
						Resources: &v1.ResourceRequirements{
							Requests: v1.ResourceList{
								"cpu":    "20m",
								"memory": "256M",
							},
							Limits: v1.ResourceList{
								"cpu":    "10m",
								"memory": "128M",
							},
						},
						Node: &v1.RadixNode{
							Gpu:      "nvidia",
							GpuCount: "6",
						},
					},
				},
			},
		}
		createdBatch := modelsV1.BatchStatus{
			JobStatus: modelsV1.JobStatus{
				Name:    "newbatch",
				Started: commonUtils.FormatTimestamp(time.Now()),
				Ended:   commonUtils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
				Status:  "batchstatus",
			},
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			CreateBatch(&batchScheduleDescription).
			Return(&createdBatch, nil).
			Times(1)
		batchHandler.
			EXPECT().
			MaintainHistoryLimit().
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(http.MethodPost, "/api/v1/batches", batchScheduleDescription)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatch modelsV1.BatchStatus
			test.GetResponseBody(response, &returnedBatch)
			assert.Equal(t, createdBatch.Name, returnedBatch.Name)
			assert.Equal(t, createdBatch.Started, returnedBatch.Started)
			assert.Equal(t, createdBatch.Ended, returnedBatch.Ended)
			assert.Equal(t, createdBatch.Status, returnedBatch.Status)
		}
	})

	t.Run("valid payload body - error from MaintainHistoryLimit should not fail request", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchScheduleDescription := models.BatchScheduleDescription{
			JobScheduleDescriptions: []models.JobScheduleDescription{
				{Payload: "a_payload"},
			},
		}
		createdBatch := modelsV1.BatchStatus{
			JobStatus: modelsV1.JobStatus{
				Name:    "newbatch",
				Started: commonUtils.FormatTimestamp(time.Now()),
				Ended:   commonUtils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
				Status:  "batchstatus",
			},
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			CreateBatch(&batchScheduleDescription).
			Return(&createdBatch, nil).
			Times(1)
		batchHandler.
			EXPECT().
			MaintainHistoryLimit().
			Return(errors.New("an error")).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(http.MethodPost, "/api/v1/batches", batchScheduleDescription)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatch modelsV1.BatchStatus
			test.GetResponseBody(response, &returnedBatch)
			assert.Equal(t, createdBatch.Name, returnedBatch.Name)
			assert.Equal(t, createdBatch.Started, returnedBatch.Started)
			assert.Equal(t, createdBatch.Ended, returnedBatch.Ended)
			assert.Equal(t, createdBatch.Status, returnedBatch.Status)
		}
	})

	t.Run("invalid request body - unprocessable", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			CreateBatch(gomock.Any()).
			Times(0)
		batchHandler.
			EXPECT().
			MaintainHistoryLimit().
			Times(0)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(http.MethodPost, "/api/v1/batches", struct{ JobScheduleDescriptions interface{} }{JobScheduleDescriptions: struct{}{}})
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusUnprocessableEntity, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonInvalid, returnedStatus.Reason)
			assert.Equal(t, apiErrors.InvalidMessage("BatchScheduleDescription", ""), returnedStatus.Message)
		}
	})

	t.Run("handler returning NotFound error - 404 not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchScheduleDescription := models.BatchScheduleDescription{}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		anyKind, anyName := "anyKind", "anyName"
		batchHandler.
			EXPECT().
			CreateBatch(&batchScheduleDescription).
			Return(nil, apiErrors.NewNotFound(anyKind, anyName)).
			Times(1)
		batchHandler.
			EXPECT().
			MaintainHistoryLimit().
			Times(0)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodPost, "/api/v1/batches")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage(anyKind, anyName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchScheduleDescription := models.BatchScheduleDescription{}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			CreateBatch(&batchScheduleDescription).
			Return(nil, errors.New("any error")).
			Times(1)
		batchHandler.
			EXPECT().
			MaintainHistoryLimit().
			Times(0)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodPost, "/api/v1/batches")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestDeleteBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			DeleteBatch(batchName).
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodDelete, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusOK, returnedStatus.Code)
			assert.Equal(t, models.StatusSuccess, returnedStatus.Status)
			assert.Empty(t, returnedStatus.Reason)
		}
	})

	t.Run("handler returning not found - 404 not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			DeleteBatch(batchName).
			Return(apiErrors.NewNotFound("batch", batchName)).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodDelete, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage("batch", batchName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			DeleteBatch(batchName).
			Return(errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodDelete, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestStopBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			StopBatch(batchName).
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/stop", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusOK, returnedStatus.Code)
			assert.Equal(t, models.StatusSuccess, returnedStatus.Status)
			assert.Empty(t, returnedStatus.Reason)
		}
	})

	t.Run("handler returning not found - 404 not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			StopBatch(batchName).
			Return(apiErrors.NewNotFound("batch", batchName)).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/stop", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage("batch", batchName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			StopBatch(batchName).
			Return(errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/stop", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestStopBatchJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		jobName := "anyjob"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			StopBatchJob(batchName, jobName).
			Return(nil).
			Times(1)
		batchHandler.
			EXPECT().
			MaintainHistoryLimit().
			Return(nil).
			AnyTimes()
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/jobs/%s/stop", batchName, jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedStatus models.Status
			err := test.GetResponseBody(response, &returnedStatus)
			if err != nil {
				panic(err)
			}
			assert.Equal(t, http.StatusOK, returnedStatus.Code)
			assert.Equal(t, models.StatusSuccess, returnedStatus.Status)
			assert.Empty(t, returnedStatus.Reason)
		}
	})

	t.Run("handler returning not found - 404 not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		jobName := "anyjob"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			StopBatchJob(batchName, jobName).
			Return(apiErrors.NewNotFound("batch", batchName)).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/jobs/%s/stop", batchName, jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage("batch", batchName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		jobName := "anyjob"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchHandler.
			EXPECT().
			StopBatchJob(batchName, jobName).
			Return(errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/jobs/%s/stop", batchName, jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}

func TestGetBatchJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "batch-name1"
		jobName := "jobname"
		jobHandler := mock.NewMockBatchHandler(ctrl)
		jobState := modelsV1.JobStatus{
			Name:    jobName,
			Started: commonUtils.FormatTimestamp(time.Now()),
			Ended:   commonUtils.FormatTimestamp(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		jobHandler.
			EXPECT().
			GetBatchJob(batchName, jobName).
			Return(&jobState, nil).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/batches/%s/jobs/%s", batchName, jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJob modelsV1.JobStatus
			test.GetResponseBody(response, &returnedJob)
			assert.Equal(t, jobState.Name, returnedJob.Name)
			assert.Equal(t, jobState.Started, returnedJob.Started)
			assert.Equal(t, jobState.Ended, returnedJob.Ended)
			assert.Equal(t, jobState.Status, returnedJob.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName, kind := "anyjob", "job"
		jobHandler := mock.NewMockBatchHandler(ctrl)
		jobHandler.
			EXPECT().
			GetBatchJob(gomock.Any(), gomock.Any()).
			Return(nil, apiErrors.NewNotFound(kind, jobName)).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/batches/%s/jobs/%s", "anybatch", "anyjob"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apiErrors.NotFoundMessage(kind, jobName), returnedStatus.Message)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := mock.NewMockBatchHandler(ctrl)
		jobHandler.
			EXPECT().
			GetBatchJob(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("unhandled error")).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(http.MethodGet, fmt.Sprintf("/api/v1/batches/%s/jobs/%s", "anybatch", "anyjob"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus models.Status
			test.GetResponseBody(response, &returnedStatus)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, models.StatusFailure, returnedStatus.Status)
			assert.Equal(t, models.StatusReasonUnknown, returnedStatus.Reason)
		}
	})
}
