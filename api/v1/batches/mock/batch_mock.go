// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/v1/batches/batch_handler.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	common "github.com/equinor/radix-job-scheduler/models/common"
	v1 "github.com/equinor/radix-job-scheduler/models/v1"
	gomock "github.com/golang/mock/gomock"
)

// MockBatchHandler is a mock of BatchHandler interface.
type MockBatchHandler struct {
	ctrl     *gomock.Controller
	recorder *MockBatchHandlerMockRecorder
}

// MockBatchHandlerMockRecorder is the mock recorder for MockBatchHandler.
type MockBatchHandlerMockRecorder struct {
	mock *MockBatchHandler
}

// NewMockBatchHandler creates a new mock instance.
func NewMockBatchHandler(ctrl *gomock.Controller) *MockBatchHandler {
	mock := &MockBatchHandler{ctrl: ctrl}
	mock.recorder = &MockBatchHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBatchHandler) EXPECT() *MockBatchHandlerMockRecorder {
	return m.recorder
}

// CopyBatch mocks base method.
func (m *MockBatchHandler) CopyBatch(arg0 context.Context, arg1, arg2 string, arg3 *common.BatchScheduleDescription) (*v1.BatchStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyBatch", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*v1.BatchStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CopyBatch indicates an expected call of CopyBatch.
func (mr *MockBatchHandlerMockRecorder) CopyBatch(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyBatch", reflect.TypeOf((*MockBatchHandler)(nil).CopyBatch), arg0, arg1, arg2, arg3)
}

// CreateBatch mocks base method.
func (m *MockBatchHandler) CreateBatch(arg0 context.Context, arg1 *common.BatchScheduleDescription) (*v1.BatchStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateBatch", arg0, arg1)
	ret0, _ := ret[0].(*v1.BatchStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateBatch indicates an expected call of CreateBatch.
func (mr *MockBatchHandlerMockRecorder) CreateBatch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateBatch", reflect.TypeOf((*MockBatchHandler)(nil).CreateBatch), arg0, arg1)
}

// DeleteBatch mocks base method.
func (m *MockBatchHandler) DeleteBatch(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteBatch", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteBatch indicates an expected call of DeleteBatch.
func (mr *MockBatchHandlerMockRecorder) DeleteBatch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteBatch", reflect.TypeOf((*MockBatchHandler)(nil).DeleteBatch), arg0, arg1)
}

// GetBatch mocks base method.
func (m *MockBatchHandler) GetBatch(arg0 context.Context, arg1 string) (*v1.BatchStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBatch", arg0, arg1)
	ret0, _ := ret[0].(*v1.BatchStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBatch indicates an expected call of GetBatch.
func (mr *MockBatchHandlerMockRecorder) GetBatch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBatch", reflect.TypeOf((*MockBatchHandler)(nil).GetBatch), arg0, arg1)
}

// GetBatchJob mocks base method.
func (m *MockBatchHandler) GetBatchJob(arg0 context.Context, arg1, arg2 string) (*v1.JobStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBatchJob", arg0, arg1, arg2)
	ret0, _ := ret[0].(*v1.JobStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBatchJob indicates an expected call of GetBatchJob.
func (mr *MockBatchHandlerMockRecorder) GetBatchJob(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBatchJob", reflect.TypeOf((*MockBatchHandler)(nil).GetBatchJob), arg0, arg1, arg2)
}

// GetBatches mocks base method.
func (m *MockBatchHandler) GetBatches(ctx context.Context) ([]v1.BatchStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBatches", ctx)
	ret0, _ := ret[0].([]v1.BatchStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBatches indicates an expected call of GetBatches.
func (mr *MockBatchHandlerMockRecorder) GetBatches(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBatches", reflect.TypeOf((*MockBatchHandler)(nil).GetBatches), ctx)
}

// MaintainHistoryLimit mocks base method.
func (m *MockBatchHandler) MaintainHistoryLimit(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaintainHistoryLimit", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// MaintainHistoryLimit indicates an expected call of MaintainHistoryLimit.
func (mr *MockBatchHandlerMockRecorder) MaintainHistoryLimit(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaintainHistoryLimit", reflect.TypeOf((*MockBatchHandler)(nil).MaintainHistoryLimit), arg0)
}

// StopBatch mocks base method.
func (m *MockBatchHandler) StopBatch(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopBatch", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopBatch indicates an expected call of StopBatch.
func (mr *MockBatchHandlerMockRecorder) StopBatch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopBatch", reflect.TypeOf((*MockBatchHandler)(nil).StopBatch), arg0, arg1)
}

// StopBatchJob mocks base method.
func (m *MockBatchHandler) StopBatchJob(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopBatchJob", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopBatchJob indicates an expected call of StopBatchJob.
func (mr *MockBatchHandlerMockRecorder) StopBatchJob(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopBatchJob", reflect.TypeOf((*MockBatchHandler)(nil).StopBatchJob), arg0, arg1, arg2)
}
