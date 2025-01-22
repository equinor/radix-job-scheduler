// Code generated by MockGen. DO NOT EDIT.
// Source: ./pkg/batch/history.go

// Package batch is a generated GoMock package.
package batch

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockHistory is a mock of History interface.
type MockHistory struct {
	ctrl     *gomock.Controller
	recorder *MockHistoryMockRecorder
}

// MockHistoryMockRecorder is the mock recorder for MockHistory.
type MockHistoryMockRecorder struct {
	mock *MockHistory
}

// NewMockHistory creates a new mock instance.
func NewMockHistory(ctrl *gomock.Controller) *MockHistory {
	mock := &MockHistory{ctrl: ctrl}
	mock.recorder = &MockHistoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHistory) EXPECT() *MockHistoryMockRecorder {
	return m.recorder
}

// Cleanup mocks base method.
func (m *MockHistory) Cleanup(ctx context.Context, namesMap map[string]struct{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Cleanup", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Cleanup indicates an expected call of Cleanup.
func (mr *MockHistoryMockRecorder) Cleanup(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Cleanup", reflect.TypeOf((*MockHistory)(nil).Cleanup), ctx)
}
