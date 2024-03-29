// Code generated by MockGen. DO NOT EDIT.
// Source: provider.go

// Package mock_metrics is a generated GoMock package.
package mock_metrics

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	metrics "github.com/k-yomo/elastic-cloud-autoscaler/metrics"
)

// MockProvider is a mock of Provider interface.
type MockProvider struct {
	ctrl     *gomock.Controller
	recorder *MockProviderMockRecorder
}

// MockProviderMockRecorder is the mock recorder for MockProvider.
type MockProviderMockRecorder struct {
	mock *MockProvider
}

// NewMockProvider creates a new mock instance.
func NewMockProvider(ctrl *gomock.Controller) *MockProvider {
	mock := &MockProvider{ctrl: ctrl}
	mock.recorder = &MockProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockProvider) EXPECT() *MockProviderMockRecorder {
	return m.recorder
}

// GetCPUUtilMetrics mocks base method.
func (m *MockProvider) GetCPUUtilMetrics(ctx context.Context, targetNodeNames []string, after time.Time) (metrics.AvgCPUUtils, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCPUUtilMetrics", ctx, targetNodeNames, after)
	ret0, _ := ret[0].(metrics.AvgCPUUtils)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCPUUtilMetrics indicates an expected call of GetCPUUtilMetrics.
func (mr *MockProviderMockRecorder) GetCPUUtilMetrics(ctx, targetNodeNames, after interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCPUUtilMetrics", reflect.TypeOf((*MockProvider)(nil).GetCPUUtilMetrics), ctx, targetNodeNames, after)
}
