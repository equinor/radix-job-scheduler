package batch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	apierrors "github.com/equinor/radix-job-scheduler/api/errors"
	api "github.com/equinor/radix-job-scheduler/api/v1/batches"
	"github.com/equinor/radix-job-scheduler/api/v1/batches/mock"
	"github.com/equinor/radix-job-scheduler/internal/test"
	models "github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(handler api.BatchHandler) *test.ControllerTestUtils {
	controller := batchController{handler: handler}
	controllerTestUtils := test.NewControllerTestUtils(&controller)
	return &controllerTestUtils
}

func TestGetBatches(t *testing.T) {
	t.Run("Get batches - success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchHandler := mock.NewMockBatchHandler(ctrl)
		batchState := modelsv1.BatchStatus{
			Name:      "batchname",
			Started:   pointers.Ptr(time.Now()),
			Ended:     pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:    "batchstatus",
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		ctx := context.Background()
		batchHandler.
			EXPECT().
			GetBatches(test.RequestContextMatcher{}).
			Return([]modelsv1.BatchStatus{batchState}, nil).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, "api/v1/batches")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatches []modelsv1.BatchStatus
			err := test.GetResponseBody(response, &returnedBatches)
			require.NoError(t, err)
			assert.Len(t, returnedBatches, 1)
			assert.Equal(t, batchState.Name, returnedBatches[0].Name)
			assert.WithinDuration(t, *batchState.Started, *returnedBatches[0].Started, 1)
			assert.WithinDuration(t, *batchState.Ended, *returnedBatches[0].Ended, 1)
			assert.Equal(t, batchState.Status, returnedBatches[0].Status)
		}
	})

	t.Run("Get batches - status code 500", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			GetBatches(test.RequestContextMatcher{}).
			Return(nil, apierrors.NewInternalError(fmt.Errorf("unhandled error"))).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, "api/v1/batches")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonInternalError, returnedStatus.Reason)
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
		batchState := modelsv1.BatchStatus{
			Name:      batchName,
			Started:   pointers.Ptr(time.Now()),
			Ended:     pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:    "batchstatus",
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		ctx := context.Background()
		batchHandler.
			EXPECT().
			GetBatch(test.RequestContextMatcher{}, batchName).
			Return(&batchState, nil).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatch modelsv1.BatchStatus
			err := test.GetResponseBody(response, &returnedBatch)
			require.NoError(t, err)
			assert.Equal(t, batchState.Name, returnedBatch.Name)
			assert.WithinDuration(t, *batchState.Started, *returnedBatch.Started, 1)
			assert.WithinDuration(t, *batchState.Ended, *returnedBatch.Ended, 1)
			assert.Equal(t, batchState.Status, returnedBatch.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName, kind := "anybatch", "batch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			GetBatch(test.RequestContextMatcher{}, gomock.Any()).
			Return(nil, apierrors.NewNotFoundError(kind, batchName, nil)).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apierrors.NotFoundMessage(kind, batchName), returnedStatus.Message)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			GetBatch(test.RequestContextMatcher{}, gomock.Any()).
			Return(nil, errors.New("unhandled error")).
			Times(1)

		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/batches/%s", "anybatch"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonInternalError, returnedStatus.Reason)
		}
	})
}

func TestCreateBatch(t *testing.T) {
	t.Run("empty body - successful", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchScheduleDescription := models.BatchScheduleDescription{}
		createdBatch := modelsv1.BatchStatus{
			Name:      "newbatch",
			Started:   pointers.Ptr(time.Now()),
			Ended:     pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:    "batchstatus",
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			CreateBatch(test.RequestContextMatcher{}, batchScheduleDescription).
			Return(&createdBatch, nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(ctx, http.MethodPost, "/api/v1/batches", nil)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatch modelsv1.BatchStatus
			err := test.GetResponseBody(response, &returnedBatch)
			require.NoError(t, err)
			assert.Equal(t, createdBatch.Name, returnedBatch.Name)
			assert.WithinDuration(t, *createdBatch.Started, *returnedBatch.Started, 1)
			assert.WithinDuration(t, *createdBatch.Ended, *returnedBatch.Ended, 1)
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
						Resources: &models.Resources{
							Requests: models.ResourceList{
								"cpu":    "20m",
								"memory": "256M",
							},
							Limits: models.ResourceList{
								"cpu":    "10m",
								"memory": "128M",
							},
						},
						Node: &models.Node{
							Gpu:      "nvidia",
							GpuCount: "6",
						},
					},
				},
			},
		}
		createdBatch := modelsv1.BatchStatus{
			Name:      "newbatch",
			Started:   pointers.Ptr(time.Now()),
			Ended:     pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:    "batchstatus",
			BatchType: string(kube.RadixBatchTypeBatch),
		}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			CreateBatch(test.RequestContextMatcher{}, batchScheduleDescription).
			Return(&createdBatch, nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(ctx, http.MethodPost, "/api/v1/batches", batchScheduleDescription)
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedBatch modelsv1.BatchStatus
			err := test.GetResponseBody(response, &returnedBatch)
			require.NoError(t, err)
			assert.Equal(t, createdBatch.Name, returnedBatch.Name)
			assert.WithinDuration(t, *createdBatch.Started, *returnedBatch.Started, 1)
			assert.WithinDuration(t, *createdBatch.Ended, *returnedBatch.Ended, 1)
			assert.Equal(t, createdBatch.Status, returnedBatch.Status)
		}
	})

	t.Run("invalid request body - unprocessable", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			CreateBatch(test.RequestContextMatcher{}, gomock.Any()).
			Times(0)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequestWithBody(ctx, http.MethodPost, "/api/v1/batches", struct{ JobScheduleDescriptions interface{} }{JobScheduleDescriptions: struct{}{}})
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnprocessableEntity, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonInvalid, returnedStatus.Reason)
			assert.Equal(t, apierrors.InvalidMessage("BatchScheduleDescription", ""), returnedStatus.Message)
		}
	})

	t.Run("handler returning NotFound error - 404 not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchScheduleDescription := models.BatchScheduleDescription{}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		anyKind, anyName := "anyKind", "anyName"
		ctx := context.Background()
		batchHandler.
			EXPECT().
			CreateBatch(test.RequestContextMatcher{}, batchScheduleDescription).
			Return(nil, apierrors.NewNotFoundError(anyKind, anyName, nil)).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, "/api/v1/batches")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apierrors.NotFoundMessage(anyKind, anyName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchScheduleDescription := models.BatchScheduleDescription{}
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			CreateBatch(test.RequestContextMatcher{}, batchScheduleDescription).
			Return(nil, errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, "/api/v1/batches")
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonInternalError, returnedStatus.Reason)
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
		ctx := context.Background()
		batchHandler.
			EXPECT().
			DeleteBatch(test.RequestContextMatcher{}, batchName).
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusSuccess, returnedStatus.Status)
			assert.Empty(t, returnedStatus.Reason)
		}
	})

	t.Run("handler returning not found - 404 not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			DeleteBatch(test.RequestContextMatcher{}, batchName).
			Return(apierrors.NewNotFoundError("batch", batchName, nil)).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apierrors.NotFoundMessage("batch", batchName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			DeleteBatch(test.RequestContextMatcher{}, batchName).
			Return(errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/batches/%s", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonInternalError, returnedStatus.Reason)
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
		ctx := context.Background()
		batchHandler.
			EXPECT().
			StopBatch(test.RequestContextMatcher{}, batchName).
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/stop", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusSuccess, returnedStatus.Status)
			assert.Empty(t, returnedStatus.Reason)
		}
	})

	t.Run("handler returning not found - 404 not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			StopBatch(test.RequestContextMatcher{}, batchName).
			Return(apierrors.NewNotFoundError("batch", batchName, nil)).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/stop", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apierrors.NotFoundMessage("batch", batchName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			StopBatch(test.RequestContextMatcher{}, batchName).
			Return(errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/stop", batchName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonInternalError, returnedStatus.Reason)
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
		ctx := context.Background()
		batchHandler.
			EXPECT().
			StopBatchJob(test.RequestContextMatcher{}, batchName, jobName).
			Return(nil).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/jobs/%s/stop", batchName, jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusSuccess, returnedStatus.Status)
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
		ctx := context.Background()
		batchHandler.
			EXPECT().
			StopBatchJob(test.RequestContextMatcher{}, batchName, jobName).
			Return(apierrors.NewNotFoundError("batch", batchName, nil)).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/jobs/%s/stop", batchName, jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apierrors.NotFoundMessage("batch", batchName), returnedStatus.Message)
		}
	})

	t.Run("handler returning unhandled error - 500 internal server error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		batchName := "anybatch"
		jobName := "anyjob"
		batchHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		batchHandler.
			EXPECT().
			StopBatchJob(test.RequestContextMatcher{}, batchName, jobName).
			Return(errors.New("any error")).
			Times(1)
		controllerTestUtils := setupTest(batchHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/batches/%s/jobs/%s/stop", batchName, jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonInternalError, returnedStatus.Reason)
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
		jobState := modelsv1.JobStatus{
			Name:    jobName,
			Started: pointers.Ptr(time.Now()),
			Ended:   pointers.Ptr(time.Now().Add(1 * time.Minute)),
			Status:  "jobstatus",
		}
		ctx := context.Background()

		jobHandler.
			EXPECT().
			GetBatchJob(test.RequestContextMatcher{}, batchName, jobName).
			Return(&jobState, nil).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/batches/%s/jobs/%s", batchName, jobName))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusOK, response.StatusCode)
			var returnedJob modelsv1.JobStatus
			err := test.GetResponseBody(response, &returnedJob)
			require.NoError(t, err)
			assert.Equal(t, jobState.Name, returnedJob.Name)
			assert.WithinDuration(t, *jobState.Started, *returnedJob.Started, 1)
			assert.WithinDuration(t, *jobState.Ended, *returnedJob.Ended, 1)
			assert.Equal(t, jobState.Status, returnedJob.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobName, kind := "anyjob", "job"
		jobHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			GetBatchJob(test.RequestContextMatcher{}, gomock.Any(), gomock.Any()).
			Return(nil, apierrors.NewNotFoundError(kind, jobName, nil)).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/batches/%s/jobs/%s", "anybatch", "anyjob"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonNotFound, returnedStatus.Reason)
			assert.Equal(t, apierrors.NotFoundMessage(kind, jobName), returnedStatus.Message)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		jobHandler := mock.NewMockBatchHandler(ctrl)
		ctx := context.Background()
		jobHandler.
			EXPECT().
			GetBatchJob(test.RequestContextMatcher{}, gomock.Any(), gomock.Any()).
			Return(nil, errors.New("unhandled error")).
			Times(1)

		controllerTestUtils := setupTest(jobHandler)
		responseChannel := controllerTestUtils.ExecuteRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/batches/%s/jobs/%s", "anybatch", "anyjob"))
		response := <-responseChannel
		assert.NotNil(t, response)

		if response != nil {
			assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
			var returnedStatus apierrors.Status
			err := test.GetResponseBody(response, &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, returnedStatus.Code)
			assert.Equal(t, apierrors.StatusFailure, returnedStatus.Status)
			assert.Equal(t, apierrors.StatusReasonInternalError, returnedStatus.Reason)
		}
	})
}
