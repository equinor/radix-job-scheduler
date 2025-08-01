// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/v1/handlers/jobs/job_handler.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	common "github.com/equinor/radix-job-scheduler/models/common"
	v1 "github.com/equinor/radix-job-scheduler/models/v1"
	gomock "github.com/golang/mock/gomock"
)

// MockJobHandler is a mock of JobHandler interface.
type MockJobHandler struct {
	ctrl     *gomock.Controller
	recorder *MockJobHandlerMockRecorder
}

// MockJobHandlerMockRecorder is the mock recorder for MockJobHandler.
type MockJobHandlerMockRecorder struct {
	mock *MockJobHandler
}

// NewMockJobHandler creates a new mock instance.
func NewMockJobHandler(ctrl *gomock.Controller) *MockJobHandler {
	mock := &MockJobHandler{ctrl: ctrl}
	mock.recorder = &MockJobHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockJobHandler) EXPECT() *MockJobHandlerMockRecorder {
	return m.recorder
}

// CopyJob mocks base method.
func (m *MockJobHandler) CopyJob(ctx context.Context, jobName, deploymentName string) (*v1.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyJob", ctx, jobName, deploymentName)
	ret0, _ := ret[0].(*v1.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CopyJob indicates an expected call of CopyJob.
func (mr *MockJobHandlerMockRecorder) CopyJob(ctx, jobName, deploymentName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyJob", reflect.TypeOf((*MockJobHandler)(nil).CopyJob), ctx, jobName, deploymentName)
}

// CreateJob mocks base method.
func (m *MockJobHandler) CreateJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*v1.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateJob", ctx, jobScheduleDescription)
	ret0, _ := ret[0].(*v1.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateJob indicates an expected call of CreateJob.
func (mr *MockJobHandlerMockRecorder) CreateJob(ctx, jobScheduleDescription interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateJob", reflect.TypeOf((*MockJobHandler)(nil).CreateJob), ctx, jobScheduleDescription)
}

// DeleteJob mocks base method.
func (m *MockJobHandler) DeleteJob(ctx context.Context, jobName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteJob", ctx, jobName)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteJob indicates an expected call of DeleteJob.
func (mr *MockJobHandlerMockRecorder) DeleteJob(ctx, jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteJob", reflect.TypeOf((*MockJobHandler)(nil).DeleteJob), ctx, jobName)
}

// GetJob mocks base method.
func (m *MockJobHandler) GetJob(ctx context.Context, jobName string) (*v1.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetJob", ctx, jobName)
	ret0, _ := ret[0].(*v1.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetJob indicates an expected call of GetJob.
func (mr *MockJobHandlerMockRecorder) GetJob(ctx, jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetJob", reflect.TypeOf((*MockJobHandler)(nil).GetJob), ctx, jobName)
}

// GetJobs mocks base method.
func (m *MockJobHandler) GetJobs(ctx context.Context) ([]v1.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetJobs", ctx)
	ret0, _ := ret[0].([]v1.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetJobs indicates an expected call of GetJobs.
func (mr *MockJobHandlerMockRecorder) GetJobs(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetJobs", reflect.TypeOf((*MockJobHandler)(nil).GetJobs), ctx)
}

// StopAllJobs mocks base method.
func (m *MockJobHandler) StopAllJobs(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopAllJobs", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopAllJobs indicates an expected call of StopAllJobs.
func (mr *MockJobHandlerMockRecorder) StopAllJobs(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopAllJobs", reflect.TypeOf((*MockJobHandler)(nil).StopAllJobs), ctx)
}

// StopJob mocks base method.
func (m *MockJobHandler) StopJob(ctx context.Context, jobName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopJob", ctx, jobName)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopJob indicates an expected call of StopJob.
func (mr *MockJobHandlerMockRecorder) StopJob(ctx, jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopJob", reflect.TypeOf((*MockJobHandler)(nil).StopJob), ctx, jobName)
}
