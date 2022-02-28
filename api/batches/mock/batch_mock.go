// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/batches/batch.go

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	models "github.com/equinor/radix-job-scheduler/models"
	gomock "github.com/golang/mock/gomock"
)

// MockBatch is a mock of Batch interface.
type MockBatch struct {
	ctrl     *gomock.Controller
	recorder *MockBatchMockRecorder
}

// MockBatchMockRecorder is the mock recorder for MockBatch.
type MockBatchMockRecorder struct {
	mock *MockBatch
}

// NewMockBatch creates a new mock instance.
func NewMockBatch(ctrl *gomock.Controller) *MockBatch {
	mock := &MockBatch{ctrl: ctrl}
	mock.recorder = &MockBatchMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBatch) EXPECT() *MockBatchMockRecorder {
	return m.recorder
}

// CreateBatch mocks base method.
func (m *MockBatch) CreateBatch(batchScheduleDescription *models.BatchScheduleDescription) (*models.BatchStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateBatch", batchScheduleDescription)
	ret0, _ := ret[0].(*models.BatchStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateBatch indicates an expected call of CreateBatch.
func (mr *MockBatchMockRecorder) CreateBatch(batchScheduleDescription interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateBatch", reflect.TypeOf((*MockBatch)(nil).CreateBatch), batchScheduleDescription)
}

// DeleteBatch mocks base method.
func (m *MockBatch) DeleteBatch(batchName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteBatch", batchName)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteBatch indicates an expected call of DeleteBatch.
func (mr *MockBatchMockRecorder) DeleteBatch(batchName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteBatch", reflect.TypeOf((*MockBatch)(nil).DeleteBatch), batchName)
}

// GetBatch mocks base method.
func (m *MockBatch) GetBatch(batchName string) (*models.BatchStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBatch", batchName)
	ret0, _ := ret[0].(*models.BatchStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBatch indicates an expected call of GetBatch.
func (mr *MockBatchMockRecorder) GetBatch(batchName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBatch", reflect.TypeOf((*MockBatch)(nil).GetBatch), batchName)
}

// GetBatches mocks base method.
func (m *MockBatch) GetBatches() ([]models.BatchStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBatches")
	ret0, _ := ret[0].([]models.BatchStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBatches indicates an expected call of GetBatches.
func (mr *MockBatchMockRecorder) GetBatches() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBatches", reflect.TypeOf((*MockBatch)(nil).GetBatches))
}

// MaintainHistoryLimit mocks base method.
func (m *MockBatch) MaintainHistoryLimit() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaintainHistoryLimit")
	ret0, _ := ret[0].(error)
	return ret0
}

// MaintainHistoryLimit indicates an expected call of MaintainHistoryLimit.
func (mr *MockBatchMockRecorder) MaintainHistoryLimit() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaintainHistoryLimit", reflect.TypeOf((*MockBatch)(nil).MaintainHistoryLimit))
}