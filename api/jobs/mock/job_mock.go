// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/handlers/job/handler.go

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	models "github.com/equinor/radix-job-scheduler/models"
	gomock "github.com/golang/mock/gomock"
)

// MockHandler is a mock of Handler interface.
type MockHandler struct {
	ctrl     *gomock.Controller
	recorder *MockHandlerMockRecorder
}

// MockHandlerMockRecorder is the mock recorder for MockHandler.
type MockHandlerMockRecorder struct {
	mock *MockHandler
}

// NewMockHandler creates a new mock instance.
func NewMockHandler(ctrl *gomock.Controller) *MockHandler {
	mock := &MockHandler{ctrl: ctrl}
	mock.recorder = &MockHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHandler) EXPECT() *MockHandlerMockRecorder {
	return m.recorder
}

// CreateJob mocks base method.
func (m *MockHandler) CreateJob(jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateJob", jobScheduleDescription)
	ret0, _ := ret[0].(*models.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateJob indicates an expected call of CreateJob.
func (mr *MockHandlerMockRecorder) CreateJob(jobScheduleDescription interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateJob", reflect.TypeOf((*MockHandler)(nil).CreateJob), jobScheduleDescription)
}

// DeleteJob mocks base method.
func (m *MockHandler) DeleteJob(jobName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteJob", jobName)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteJob indicates an expected call of DeleteJob.
func (mr *MockHandlerMockRecorder) DeleteJob(jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteJob", reflect.TypeOf((*MockHandler)(nil).DeleteJob), jobName)
}

// GetJob mocks base method.
func (m *MockHandler) GetJob(name string) (*models.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetJob", name)
	ret0, _ := ret[0].(*models.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetJob indicates an expected call of GetJob.
func (mr *MockHandlerMockRecorder) GetJob(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetJob", reflect.TypeOf((*MockHandler)(nil).GetJob), name)
}

// GetJobs mocks base method.
func (m *MockHandler) GetJobs() ([]models.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetJobs")
	ret0, _ := ret[0].([]models.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetJobs indicates an expected call of GetJobs.
func (mr *MockHandlerMockRecorder) GetJobs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetJobs", reflect.TypeOf((*MockHandler)(nil).GetJobs))
}

// MaintainHistoryLimit mocks base method.
func (m *MockHandler) MaintainHistoryLimit() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaintainHistoryLimit")
	ret0, _ := ret[0].(error)
	return ret0
}

// MaintainHistoryLimit indicates an expected call of MaintainHistoryLimit.
func (mr *MockHandlerMockRecorder) MaintainHistoryLimit() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaintainHistoryLimit", reflect.TypeOf((*MockHandler)(nil).MaintainHistoryLimit))
}
