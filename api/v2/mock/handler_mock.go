// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/v2/handler.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
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
func (m *MockHandler) CopyRadixBatch(arg0 context.Context, arg1, arg2 string) (*v2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyRadixBatch", arg0, arg1, arg2)
	ret0, _ := ret[0].(*v2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CopyRadixBatch indicates an expected call of CopyRadixBatch.
func (mr *MockHandlerMockRecorder) CopyRadixBatch(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyRadixBatch", reflect.TypeOf((*MockHandler)(nil).CopyRadixBatch), arg0, arg1, arg2)
}

// CopyRadixBatchSingleJob mocks base method.
func (m *MockHandler) CopyRadixBatchSingleJob(arg0 context.Context, arg1, arg2, arg3 string) (*v2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyRadixBatchSingleJob", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*v2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CopyRadixBatchSingleJob indicates an expected call of CopyRadixBatchSingleJob.
func (mr *MockHandlerMockRecorder) CopyRadixBatchSingleJob(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyRadixBatchSingleJob", reflect.TypeOf((*MockHandler)(nil).CopyRadixBatchSingleJob), arg0, arg1, arg2, arg3)
}

// CreateRadixBatch mocks base method.
func (m *MockHandler) CreateRadixBatch(arg0 context.Context, arg1 *common.BatchScheduleDescription) (*v2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRadixBatch", arg0, arg1)
	ret0, _ := ret[0].(*v2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateRadixBatch indicates an expected call of CreateRadixBatch.
func (mr *MockHandlerMockRecorder) CreateRadixBatch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRadixBatch", reflect.TypeOf((*MockHandler)(nil).CreateRadixBatch), arg0, arg1)
}

// CreateRadixBatchSingleJob mocks base method.
func (m *MockHandler) CreateRadixBatchSingleJob(arg0 context.Context, arg1 *common.JobScheduleDescription) (*v2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRadixBatchSingleJob", arg0, arg1)
	ret0, _ := ret[0].(*v2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateRadixBatchSingleJob indicates an expected call of CreateRadixBatchSingleJob.
func (mr *MockHandlerMockRecorder) CreateRadixBatchSingleJob(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRadixBatchSingleJob", reflect.TypeOf((*MockHandler)(nil).CreateRadixBatchSingleJob), arg0, arg1)
}

// DeleteRadixBatch mocks base method.
func (m *MockHandler) DeleteRadixBatch(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteRadixBatch", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteRadixBatch indicates an expected call of DeleteRadixBatch.
func (mr *MockHandlerMockRecorder) DeleteRadixBatch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteRadixBatch", reflect.TypeOf((*MockHandler)(nil).DeleteRadixBatch), arg0, arg1)
}

// GarbageCollectPayloadSecrets mocks base method.
func (m *MockHandler) GarbageCollectPayloadSecrets(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GarbageCollectPayloadSecrets", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// GarbageCollectPayloadSecrets indicates an expected call of GarbageCollectPayloadSecrets.
func (mr *MockHandlerMockRecorder) GarbageCollectPayloadSecrets(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GarbageCollectPayloadSecrets", reflect.TypeOf((*MockHandler)(nil).GarbageCollectPayloadSecrets), arg0)
}

// GetCompletedRadixBatchesSortedByCompletionTimeAsc mocks base method.
func (m *MockHandler) GetCompletedRadixBatchesSortedByCompletionTimeAsc(arg0 context.Context) (*apiv2.CompletedRadixBatches, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCompletedRadixBatchesSortedByCompletionTimeAsc", arg0)
	ret0, _ := ret[0].(*apiv2.CompletedRadixBatches)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCompletedRadixBatchesSortedByCompletionTimeAsc indicates an expected call of GetCompletedRadixBatchesSortedByCompletionTimeAsc.
func (mr *MockHandlerMockRecorder) GetCompletedRadixBatchesSortedByCompletionTimeAsc(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCompletedRadixBatchesSortedByCompletionTimeAsc", reflect.TypeOf((*MockHandler)(nil).GetCompletedRadixBatchesSortedByCompletionTimeAsc), arg0)
}

// GetRadixBatch mocks base method.
func (m *MockHandler) GetRadixBatch(arg0 context.Context, arg1 string) (*v2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatch", arg0, arg1)
	ret0, _ := ret[0].(*v2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatch indicates an expected call of GetRadixBatch.
func (mr *MockHandlerMockRecorder) GetRadixBatch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatch", reflect.TypeOf((*MockHandler)(nil).GetRadixBatch), arg0, arg1)
}

// GetRadixBatchSingleJobs mocks base method.
func (m *MockHandler) GetRadixBatchSingleJobs(arg0 context.Context) ([]v2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatchSingleJobs", arg0)
	ret0, _ := ret[0].([]v2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatchSingleJobs indicates an expected call of GetRadixBatchSingleJobs.
func (mr *MockHandlerMockRecorder) GetRadixBatchSingleJobs(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatchSingleJobs", reflect.TypeOf((*MockHandler)(nil).GetRadixBatchSingleJobs), arg0)
}

// GetRadixBatches mocks base method.
func (m *MockHandler) GetRadixBatches(arg0 context.Context) ([]v2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatches", arg0)
	ret0, _ := ret[0].([]v2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatches indicates an expected call of GetRadixBatches.
func (mr *MockHandlerMockRecorder) GetRadixBatches(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatches", reflect.TypeOf((*MockHandler)(nil).GetRadixBatches), arg0)
}

// MaintainHistoryLimit mocks base method.
func (m *MockHandler) MaintainHistoryLimit(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaintainHistoryLimit", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// MaintainHistoryLimit indicates an expected call of MaintainHistoryLimit.
func (mr *MockHandlerMockRecorder) MaintainHistoryLimit(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaintainHistoryLimit", reflect.TypeOf((*MockHandler)(nil).MaintainHistoryLimit), arg0)
}

// StopRadixBatch mocks base method.
func (m *MockHandler) StopRadixBatch(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopRadixBatch", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopRadixBatch indicates an expected call of StopRadixBatch.
func (mr *MockHandlerMockRecorder) StopRadixBatch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopRadixBatch", reflect.TypeOf((*MockHandler)(nil).StopRadixBatch), arg0, arg1)
}

// StopRadixBatchJob mocks base method.
func (m *MockHandler) StopRadixBatchJob(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopRadixBatchJob", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopRadixBatchJob indicates an expected call of StopRadixBatchJob.
func (mr *MockHandlerMockRecorder) StopRadixBatchJob(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopRadixBatchJob", reflect.TypeOf((*MockHandler)(nil).StopRadixBatchJob), arg0, arg1, arg2)
}
