// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/mbrt/k8cc/pkg/controller (interfaces: Controller,TagController,ScaleSettingsProvider)

// Package mock_controller is a generated GoMock package.
package mock_controller

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	controller "github.com/mbrt/k8cc/pkg/controller"
	data "github.com/mbrt/k8cc/pkg/data"
	reflect "reflect"
	time "time"
)

// MockController is a mock of Controller interface
type MockController struct {
	ctrl     *gomock.Controller
	recorder *MockControllerMockRecorder
}

// MockControllerMockRecorder is the mock recorder for MockController
type MockControllerMockRecorder struct {
	mock *MockController
}

// NewMockController creates a new mock instance
func NewMockController(ctrl *gomock.Controller) *MockController {
	mock := &MockController{ctrl: ctrl}
	mock.recorder = &MockControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockController) EXPECT() *MockControllerMockRecorder {
	return m.recorder
}

// TagController mocks base method
func (m *MockController) TagController(arg0 data.Tag) controller.TagController {
	ret := m.ctrl.Call(m, "TagController", arg0)
	ret0, _ := ret[0].(controller.TagController)
	return ret0
}

// TagController indicates an expected call of TagController
func (mr *MockControllerMockRecorder) TagController(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TagController", reflect.TypeOf((*MockController)(nil).TagController), arg0)
}

// MockTagController is a mock of TagController interface
type MockTagController struct {
	ctrl     *gomock.Controller
	recorder *MockTagControllerMockRecorder
}

// MockTagControllerMockRecorder is the mock recorder for MockTagController
type MockTagControllerMockRecorder struct {
	mock *MockTagController
}

// NewMockTagController creates a new mock instance
func NewMockTagController(ctrl *gomock.Controller) *MockTagController {
	mock := &MockTagController{ctrl: ctrl}
	mock.recorder = &MockTagControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockTagController) EXPECT() *MockTagControllerMockRecorder {
	return m.recorder
}

// DesiredReplicas mocks base method
func (m *MockTagController) DesiredReplicas(arg0 time.Time) int {
	ret := m.ctrl.Call(m, "DesiredReplicas", arg0)
	ret0, _ := ret[0].(int)
	return ret0
}

// DesiredReplicas indicates an expected call of DesiredReplicas
func (mr *MockTagControllerMockRecorder) DesiredReplicas(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DesiredReplicas", reflect.TypeOf((*MockTagController)(nil).DesiredReplicas), arg0)
}

// LeaseUser mocks base method
func (m *MockTagController) LeaseUser(arg0 context.Context, arg1 data.User, arg2 time.Time) (controller.Lease, error) {
	ret := m.ctrl.Call(m, "LeaseUser", arg0, arg1, arg2)
	ret0, _ := ret[0].(controller.Lease)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LeaseUser indicates an expected call of LeaseUser
func (mr *MockTagControllerMockRecorder) LeaseUser(arg0, arg1, arg2 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LeaseUser", reflect.TypeOf((*MockTagController)(nil).LeaseUser), arg0, arg1, arg2)
}

// MockScaleSettingsProvider is a mock of ScaleSettingsProvider interface
type MockScaleSettingsProvider struct {
	ctrl     *gomock.Controller
	recorder *MockScaleSettingsProviderMockRecorder
}

// MockScaleSettingsProviderMockRecorder is the mock recorder for MockScaleSettingsProvider
type MockScaleSettingsProviderMockRecorder struct {
	mock *MockScaleSettingsProvider
}

// NewMockScaleSettingsProvider creates a new mock instance
func NewMockScaleSettingsProvider(ctrl *gomock.Controller) *MockScaleSettingsProvider {
	mock := &MockScaleSettingsProvider{ctrl: ctrl}
	mock.recorder = &MockScaleSettingsProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockScaleSettingsProvider) EXPECT() *MockScaleSettingsProviderMockRecorder {
	return m.recorder
}

// ScaleSettings mocks base method
func (m *MockScaleSettingsProvider) ScaleSettings(arg0 data.Tag) (data.ScaleSettings, error) {
	ret := m.ctrl.Call(m, "ScaleSettings", arg0)
	ret0, _ := ret[0].(data.ScaleSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ScaleSettings indicates an expected call of ScaleSettings
func (mr *MockScaleSettingsProviderMockRecorder) ScaleSettings(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScaleSettings", reflect.TypeOf((*MockScaleSettingsProvider)(nil).ScaleSettings), arg0)
}
