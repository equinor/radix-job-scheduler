// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/v1/jobs/job_handler.go

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

// CreateJob mocks base method.
func (m *MockJobHandler) CreateJob(arg0 context.Context, arg1 *common.JobScheduleDescription) (*v1.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateJob", arg0, arg1)
	ret0, _ := ret[0].(*v1.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateJob indicates an expected call of CreateJob.
func (mr *MockJobHandlerMockRecorder) CreateJob(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateJob", reflect.TypeOf((*MockJobHandler)(nil).CreateJob), arg0, arg1)
}

// DeleteJob mocks base method.
func (m *MockJobHandler) DeleteJob(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteJob", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteJob indicates an expected call of DeleteJob.
func (mr *MockJobHandlerMockRecorder) DeleteJob(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteJob", reflect.TypeOf((*MockJobHandler)(nil).DeleteJob), arg0, arg1)
}

// GetJob mocks base method.
func (m *MockJobHandler) GetJob(arg0 context.Context, arg1 string) (*v1.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetJob", arg0, arg1)
	ret0, _ := ret[0].(*v1.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetJob indicates an expected call of GetJob.
func (mr *MockJobHandlerMockRecorder) GetJob(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetJob", reflect.TypeOf((*MockJobHandler)(nil).GetJob), arg0, arg1)
}

// GetJobs mocks base method.
func (m *MockJobHandler) GetJobs(arg0 context.Context) ([]v1.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetJobs", arg0)
	ret0, _ := ret[0].([]v1.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetJobs indicates an expected call of GetJobs.
func (mr *MockJobHandlerMockRecorder) GetJobs(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetJobs", reflect.TypeOf((*MockJobHandler)(nil).GetJobs), arg0)
}

// MaintainHistoryLimit mocks base method.
func (m *MockJobHandler) MaintainHistoryLimit(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaintainHistoryLimit", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// MaintainHistoryLimit indicates an expected call of MaintainHistoryLimit.
func (mr *MockJobHandlerMockRecorder) MaintainHistoryLimit(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaintainHistoryLimit", reflect.TypeOf((*MockJobHandler)(nil).MaintainHistoryLimit), arg0)
}

// StopJob mocks base method.
func (m *MockJobHandler) StopJob(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopJob", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopJob indicates an expected call of StopJob.
func (mr *MockJobHandlerMockRecorder) StopJob(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopJob", reflect.TypeOf((*MockJobHandler)(nil).StopJob), arg0, arg1)
}
