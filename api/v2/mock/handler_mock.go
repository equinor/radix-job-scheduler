// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/v2/handler.go

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	models "github.com/equinor/radix-job-scheduler/models"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
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

// CreateRadixBatch mocks base method.
func (m *MockHandler) CreateRadixBatch(arg0 *models.BatchScheduleDescription) (*modelsv2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRadixBatch", arg0)
	ret0, _ := ret[0].(*modelsv2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateRadixBatch indicates an expected call of CreateRadixBatch.
func (mr *MockHandlerMockRecorder) CreateRadixBatch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRadixBatch", reflect.TypeOf((*MockHandler)(nil).CreateRadixBatch), arg0)
}

// CreateRadixBatchSingleJob mocks base method.
func (m *MockHandler) CreateRadixBatchSingleJob(arg0 *models.JobScheduleDescription) (*modelsv2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRadixBatchSingleJob", arg0)
	ret0, _ := ret[0].(*modelsv2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateRadixBatchSingleJob indicates an expected call of CreateRadixBatchSingleJob.
func (mr *MockHandlerMockRecorder) CreateRadixBatchSingleJob(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRadixBatchSingleJob", reflect.TypeOf((*MockHandler)(nil).CreateRadixBatchSingleJob), arg0)
}

// DeleteRadixBatch mocks base method.
func (m *MockHandler) DeleteRadixBatch(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteRadixBatch", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteRadixBatch indicates an expected call of DeleteRadixBatch.
func (mr *MockHandlerMockRecorder) DeleteRadixBatch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteRadixBatch", reflect.TypeOf((*MockHandler)(nil).DeleteRadixBatch), arg0)
}

// GarbageCollectPayloadSecrets mocks base method.
func (m *MockHandler) GarbageCollectPayloadSecrets() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GarbageCollectPayloadSecrets")
	ret0, _ := ret[0].(error)
	return ret0
}

// GarbageCollectPayloadSecrets indicates an expected call of GarbageCollectPayloadSecrets.
func (mr *MockHandlerMockRecorder) GarbageCollectPayloadSecrets() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GarbageCollectPayloadSecrets", reflect.TypeOf((*MockHandler)(nil).GarbageCollectPayloadSecrets))
}

// GetCompletedRadixBatchesSortedByCompletionTimeAsc mocks base method.
func (m *MockHandler) GetCompletedRadixBatchesSortedByCompletionTimeAsc() (*apiv2.CompletedRadixBatches, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCompletedRadixBatchesSortedByCompletionTimeAsc")
	ret0, _ := ret[0].(*apiv2.CompletedRadixBatches)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCompletedRadixBatchesSortedByCompletionTimeAsc indicates an expected call of GetCompletedRadixBatchesSortedByCompletionTimeAsc.
func (mr *MockHandlerMockRecorder) GetCompletedRadixBatchesSortedByCompletionTimeAsc() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCompletedRadixBatchesSortedByCompletionTimeAsc", reflect.TypeOf((*MockHandler)(nil).GetCompletedRadixBatchesSortedByCompletionTimeAsc))
}

// GetRadixBatch mocks base method.
func (m *MockHandler) GetRadixBatch(arg0 string) (*modelsv2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatch", arg0)
	ret0, _ := ret[0].(*modelsv2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatch indicates an expected call of GetRadixBatch.
func (mr *MockHandlerMockRecorder) GetRadixBatch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatch", reflect.TypeOf((*MockHandler)(nil).GetRadixBatch), arg0)
}

// GetRadixBatchSingleJobs mocks base method.
func (m *MockHandler) GetRadixBatchSingleJobs() ([]modelsv2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatchSingleJobs")
	ret0, _ := ret[0].([]modelsv2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatchSingleJobs indicates an expected call of GetRadixBatchSingleJobs.
func (mr *MockHandlerMockRecorder) GetRadixBatchSingleJobs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatchSingleJobs", reflect.TypeOf((*MockHandler)(nil).GetRadixBatchSingleJobs))
}

// GetRadixBatches mocks base method.
func (m *MockHandler) GetRadixBatches() ([]modelsv2.RadixBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRadixBatches")
	ret0, _ := ret[0].([]modelsv2.RadixBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRadixBatches indicates an expected call of GetRadixBatches.
func (mr *MockHandlerMockRecorder) GetRadixBatches() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRadixBatches", reflect.TypeOf((*MockHandler)(nil).GetRadixBatches))
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

// StopRadixBatch mocks base method.
func (m *MockHandler) StopRadixBatch(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopRadixBatch", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopRadixBatch indicates an expected call of StopRadixBatch.
func (mr *MockHandlerMockRecorder) StopRadixBatch(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopRadixBatch", reflect.TypeOf((*MockHandler)(nil).StopRadixBatch), arg0)
}

// StopRadixBatchJob mocks base method.
func (m *MockHandler) StopRadixBatchJob(arg0, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopRadixBatchJob", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopRadixBatchJob indicates an expected call of StopRadixBatchJob.
func (mr *MockHandlerMockRecorder) StopRadixBatchJob(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopRadixBatchJob", reflect.TypeOf((*MockHandler)(nil).StopRadixBatchJob), arg0, arg1)
}
