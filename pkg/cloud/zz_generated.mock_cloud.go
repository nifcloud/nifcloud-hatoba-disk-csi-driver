// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/cloud/cloud.go

// Package cloud is a generated GoMock package.
package cloud

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockCloud is a mock of Cloud interface.
type MockCloud struct {
	ctrl     *gomock.Controller
	recorder *MockCloudMockRecorder
}

// MockCloudMockRecorder is the mock recorder for MockCloud.
type MockCloudMockRecorder struct {
	mock *MockCloud
}

// NewMockCloud creates a new mock instance.
func NewMockCloud(ctrl *gomock.Controller) *MockCloud {
	mock := &MockCloud{ctrl: ctrl}
	mock.recorder = &MockCloudMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCloud) EXPECT() *MockCloudMockRecorder {
	return m.recorder
}

// AttachDisk mocks base method.
func (m *MockCloud) AttachDisk(ctx context.Context, volumeID, nodeID string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AttachDisk", ctx, volumeID, nodeID)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AttachDisk indicates an expected call of AttachDisk.
func (mr *MockCloudMockRecorder) AttachDisk(ctx, volumeID, nodeID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AttachDisk", reflect.TypeOf((*MockCloud)(nil).AttachDisk), ctx, volumeID, nodeID)
}

// CreateDisk mocks base method.
func (m *MockCloud) CreateDisk(ctx context.Context, clusterName string, diskOptions *DiskOptions) (*Volume, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDisk", ctx, clusterName, diskOptions)
	ret0, _ := ret[0].(*Volume)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDisk indicates an expected call of CreateDisk.
func (mr *MockCloudMockRecorder) CreateDisk(ctx, clusterName, diskOptions interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDisk", reflect.TypeOf((*MockCloud)(nil).CreateDisk), ctx, clusterName, diskOptions)
}

// DeleteDisk mocks base method.
func (m *MockCloud) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteDisk", ctx, volumeID)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteDisk indicates an expected call of DeleteDisk.
func (mr *MockCloudMockRecorder) DeleteDisk(ctx, volumeID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDisk", reflect.TypeOf((*MockCloud)(nil).DeleteDisk), ctx, volumeID)
}

// DetachDisk mocks base method.
func (m *MockCloud) DetachDisk(ctx context.Context, volumeID, nodeID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DetachDisk", ctx, volumeID, nodeID)
	ret0, _ := ret[0].(error)
	return ret0
}

// DetachDisk indicates an expected call of DetachDisk.
func (mr *MockCloudMockRecorder) DetachDisk(ctx, volumeID, nodeID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DetachDisk", reflect.TypeOf((*MockCloud)(nil).DetachDisk), ctx, volumeID, nodeID)
}

// GetClusterName mocks base method.
func (m *MockCloud) GetClusterName(ctx context.Context, nrn string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClusterName", ctx, nrn)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetClusterName indicates an expected call of GetClusterName.
func (mr *MockCloudMockRecorder) GetClusterName(ctx, nrn interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClusterName", reflect.TypeOf((*MockCloud)(nil).GetClusterName), ctx, nrn)
}

// GetDiskByID mocks base method.
func (m *MockCloud) GetDiskByID(ctx context.Context, volumeID string) (*Volume, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDiskByID", ctx, volumeID)
	ret0, _ := ret[0].(*Volume)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDiskByID indicates an expected call of GetDiskByID.
func (mr *MockCloudMockRecorder) GetDiskByID(ctx, volumeID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDiskByID", reflect.TypeOf((*MockCloud)(nil).GetDiskByID), ctx, volumeID)
}

// GetDiskByName mocks base method.
func (m *MockCloud) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*Volume, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDiskByName", ctx, name, capacityBytes)
	ret0, _ := ret[0].(*Volume)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDiskByName indicates an expected call of GetDiskByName.
func (mr *MockCloudMockRecorder) GetDiskByName(ctx, name, capacityBytes interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDiskByName", reflect.TypeOf((*MockCloud)(nil).GetDiskByName), ctx, name, capacityBytes)
}

// IsExistNode mocks base method.
func (m *MockCloud) IsExistNode(ctx context.Context, clusterName, nodeID string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsExistNode", ctx, clusterName, nodeID)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsExistNode indicates an expected call of IsExistNode.
func (mr *MockCloudMockRecorder) IsExistNode(ctx, clusterName, nodeID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsExistNode", reflect.TypeOf((*MockCloud)(nil).IsExistNode), ctx, clusterName, nodeID)
}

// ResizeDisk mocks base method.
func (m *MockCloud) ResizeDisk(ctx context.Context, volumeID string, size int64) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResizeDisk", ctx, volumeID, size)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ResizeDisk indicates an expected call of ResizeDisk.
func (mr *MockCloudMockRecorder) ResizeDisk(ctx, volumeID, size interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResizeDisk", reflect.TypeOf((*MockCloud)(nil).ResizeDisk), ctx, volumeID, size)
}
