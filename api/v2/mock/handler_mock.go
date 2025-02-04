// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/v2/handler.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	common "github.com/equinor/radix-job-scheduler/models/common"
	v2 "github.com/equinor/radix-job-scheduler/models/v2"
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

// CopyRadixBatch mocks base method.
func (m *MockHandler) CopyRadixBatch(ctx context.Context, batchName, deploymentName string) (*v2.Batch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyRadixBatch", ctx, batchName, deploymentName)
	ret0, _ := ret[0].(*v2.Batch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CopyRadixBatch indicates an expected call of CopyRadixBatch.
func (mr *MockHandlerMockRecorder) CopyRadixBatch(ctx, batchName, deploymentName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyRadixBatch", reflect.TypeOf((*MockHandler)(nil).CopyRadixBatch), ctx, batchName, deploymentName)
}

// CreateRadixBatch mocks base method.
func (m *MockHandler) CreateRadixBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*v2.Batch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRadixBatch", ctx, batchScheduleDescription)
	ret0, _ := ret[0].(*v2.Batch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateRadixBatch indicates an expected call of CreateRadixBatch.
func (mr *MockHandlerMockRecorder) CreateRadixBatch(ctx, batchScheduleDescription interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRadixBatch", reflect.TypeOf((*MockHandler)(nil).CreateRadixBatch), ctx, batchScheduleDescription)
}

// CreateRadixBatchSingleJob mocks base method.
func (m *MockHandler) CreateRadixBatchSingleJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*v2.Batch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRadixBatchSingleJob", ctx, jobScheduleDescription)
	ret0, _ := ret[0].(*v2.Batch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateRadixBatchSingleJob indicates an expected call of CreateRadixBatchSingleJob.
func (mr *MockHandlerMockRecorder) CreateRadixBatchSingleJob(ctx, jobScheduleDescription interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRadixBatchSingleJob", reflect.TypeOf((*MockHandler)(nil).CreateRadixBatchSingleJob), ctx, jobScheduleDescription)
}

// DeleteRadixBatchJob mocks base method.
func (m *MockHandler) DeleteRadixBatchJob(ctx context.Context, jobName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteRadixBatchJob", ctx, jobName)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteRadixBatchJob indicates an expected call of DeleteRadixBatchJob.
func (mr *MockHandlerMockRecorder) DeleteRadixBatchJob(ctx, jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteRadixBatchJob", reflect.TypeOf((*MockHandler)(nil).DeleteRadixBatchJob), ctx, jobName)
}

// GetRadixBatch mocks base method.
func (m *MockHandler) GetRadixBatch(ctx context.Context, batchName string) (*v2.Batch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatch", ctx, batchName)
	ret0, _ := ret[0].(*v2.Batch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatch indicates an expected call of GetRadixBatch.
func (mr *MockHandlerMockRecorder) GetRadixBatch(ctx, batchName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatch", reflect.TypeOf((*MockHandler)(nil).GetRadixBatch), ctx, batchName)
}

// GetRadixBatchSingleJobs mocks base method.
func (m *MockHandler) GetRadixBatchSingleJobs(ctx context.Context) ([]v2.Batch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatchSingleJobs", ctx)
	ret0, _ := ret[0].([]v2.Batch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatchSingleJobs indicates an expected call of GetRadixBatchSingleJobs.
func (mr *MockHandlerMockRecorder) GetRadixBatchSingleJobs(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatchSingleJobs", reflect.TypeOf((*MockHandler)(nil).GetRadixBatchSingleJobs), ctx)
}

// GetRadixBatches mocks base method.
func (m *MockHandler) GetRadixBatches(ctx context.Context) ([]v2.Batch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatches", ctx)
	ret0, _ := ret[0].([]v2.Batch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatches indicates an expected call of GetRadixBatches.
func (mr *MockHandlerMockRecorder) GetRadixBatches(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatches", reflect.TypeOf((*MockHandler)(nil).GetRadixBatches), ctx)
}

// RestartRadixBatch mocks base method.
func (m *MockHandler) RestartRadixBatch(ctx context.Context, batchName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RestartRadixBatch", ctx, batchName)
	ret0, _ := ret[0].(error)
	return ret0
}

// RestartRadixBatch indicates an expected call of RestartRadixBatch.
func (mr *MockHandlerMockRecorder) RestartRadixBatch(ctx, batchName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RestartRadixBatch", reflect.TypeOf((*MockHandler)(nil).RestartRadixBatch), ctx, batchName)
}

// RestartRadixBatchJob mocks base method.
func (m *MockHandler) RestartRadixBatchJob(ctx context.Context, batchName, jobName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RestartRadixBatchJob", ctx, batchName, jobName)
	ret0, _ := ret[0].(error)
	return ret0
}

// RestartRadixBatchJob indicates an expected call of RestartRadixBatchJob.
func (mr *MockHandlerMockRecorder) RestartRadixBatchJob(ctx, batchName, jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RestartRadixBatchJob", reflect.TypeOf((*MockHandler)(nil).RestartRadixBatchJob), ctx, batchName, jobName)
}

// StopRadixBatch mocks base method.
func (m *MockHandler) StopRadixBatch(ctx context.Context, batchName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopRadixBatch", ctx, batchName)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopRadixBatch indicates an expected call of StopRadixBatch.
func (mr *MockHandlerMockRecorder) StopRadixBatch(ctx, batchName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopRadixBatch", reflect.TypeOf((*MockHandler)(nil).StopRadixBatch), ctx, batchName)
}

// StopRadixBatchJob mocks base method.
func (m *MockHandler) StopRadixBatchJob(ctx context.Context, batchName, jobName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopRadixBatchJob", ctx, batchName, jobName)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopRadixBatchJob indicates an expected call of StopRadixBatchJob.
func (mr *MockHandlerMockRecorder) StopRadixBatchJob(ctx, batchName, jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopRadixBatchJob", reflect.TypeOf((*MockHandler)(nil).StopRadixBatchJob), ctx, batchName, jobName)
}
