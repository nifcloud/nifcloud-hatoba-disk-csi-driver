// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/driver/mount.go

// Package driver is a generated GoMock package.
package driver

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	mount_utils "k8s.io/mount-utils"
)

// MockMounter is a mock of Mounter interface.
type MockMounter struct {
	ctrl     *gomock.Controller
	recorder *MockMounterMockRecorder
}

// MockMounterMockRecorder is the mock recorder for MockMounter.
type MockMounterMockRecorder struct {
	mock *MockMounter
}

// NewMockMounter creates a new mock instance.
func NewMockMounter(ctrl *gomock.Controller) *MockMounter {
	mock := &MockMounter{ctrl: ctrl}
	mock.recorder = &MockMounterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMounter) EXPECT() *MockMounterMockRecorder {
	return m.recorder
}

// ExistsPath mocks base method.
func (m *MockMounter) ExistsPath(filename string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExistsPath", filename)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExistsPath indicates an expected call of ExistsPath.
func (mr *MockMounterMockRecorder) ExistsPath(filename interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExistsPath", reflect.TypeOf((*MockMounter)(nil).ExistsPath), filename)
}

// FindDevicePath mocks base method.
func (m *MockMounter) FindDevicePath(scsiID string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindDevicePath", scsiID)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindDevicePath indicates an expected call of FindDevicePath.
func (mr *MockMounterMockRecorder) FindDevicePath(scsiID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindDevicePath", reflect.TypeOf((*MockMounter)(nil).FindDevicePath), scsiID)
}

// FormatAndMount mocks base method.
func (m *MockMounter) FormatAndMount(source, target, fstype string, options []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FormatAndMount", source, target, fstype, options)
	ret0, _ := ret[0].(error)
	return ret0
}

// FormatAndMount indicates an expected call of FormatAndMount.
func (mr *MockMounterMockRecorder) FormatAndMount(source, target, fstype, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FormatAndMount", reflect.TypeOf((*MockMounter)(nil).FormatAndMount), source, target, fstype, options)
}

// GetDeviceName mocks base method.
func (m *MockMounter) GetDeviceName(mountPath string) (string, int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeviceName", mountPath)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(int)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetDeviceName indicates an expected call of GetDeviceName.
func (mr *MockMounterMockRecorder) GetDeviceName(mountPath interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeviceName", reflect.TypeOf((*MockMounter)(nil).GetDeviceName), mountPath)
}

// GetMountRefs mocks base method.
func (m *MockMounter) GetMountRefs(pathname string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMountRefs", pathname)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMountRefs indicates an expected call of GetMountRefs.
func (mr *MockMounterMockRecorder) GetMountRefs(pathname interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMountRefs", reflect.TypeOf((*MockMounter)(nil).GetMountRefs), pathname)
}

// IsCorruptedMnt mocks base method.
func (m *MockMounter) IsCorruptedMnt(err error) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsCorruptedMnt", err)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsCorruptedMnt indicates an expected call of IsCorruptedMnt.
func (mr *MockMounterMockRecorder) IsCorruptedMnt(err interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsCorruptedMnt", reflect.TypeOf((*MockMounter)(nil).IsCorruptedMnt), err)
}

// IsLikelyNotMountPoint mocks base method.
func (m *MockMounter) IsLikelyNotMountPoint(file string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsLikelyNotMountPoint", file)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsLikelyNotMountPoint indicates an expected call of IsLikelyNotMountPoint.
func (mr *MockMounterMockRecorder) IsLikelyNotMountPoint(file interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsLikelyNotMountPoint", reflect.TypeOf((*MockMounter)(nil).IsLikelyNotMountPoint), file)
}

// List mocks base method.
func (m *MockMounter) List() ([]mount_utils.MountPoint, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List")
	ret0, _ := ret[0].([]mount_utils.MountPoint)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockMounterMockRecorder) List() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockMounter)(nil).List))
}

// MakeDir mocks base method.
func (m *MockMounter) MakeDir(pathname string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MakeDir", pathname)
	ret0, _ := ret[0].(error)
	return ret0
}

// MakeDir indicates an expected call of MakeDir.
func (mr *MockMounterMockRecorder) MakeDir(pathname interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MakeDir", reflect.TypeOf((*MockMounter)(nil).MakeDir), pathname)
}

// MakeFile mocks base method.
func (m *MockMounter) MakeFile(pathname string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MakeFile", pathname)
	ret0, _ := ret[0].(error)
	return ret0
}

// MakeFile indicates an expected call of MakeFile.
func (mr *MockMounterMockRecorder) MakeFile(pathname interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MakeFile", reflect.TypeOf((*MockMounter)(nil).MakeFile), pathname)
}

// Mount mocks base method.
func (m *MockMounter) Mount(source, target, fstype string, options []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Mount", source, target, fstype, options)
	ret0, _ := ret[0].(error)
	return ret0
}

// Mount indicates an expected call of Mount.
func (mr *MockMounterMockRecorder) Mount(source, target, fstype, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Mount", reflect.TypeOf((*MockMounter)(nil).Mount), source, target, fstype, options)
}

// MountSensitive mocks base method.
func (m *MockMounter) MountSensitive(source, target, fstype string, options, sensitiveOptions []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountSensitive", source, target, fstype, options, sensitiveOptions)
	ret0, _ := ret[0].(error)
	return ret0
}

// MountSensitive indicates an expected call of MountSensitive.
func (mr *MockMounterMockRecorder) MountSensitive(source, target, fstype, options, sensitiveOptions interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountSensitive", reflect.TypeOf((*MockMounter)(nil).MountSensitive), source, target, fstype, options, sensitiveOptions)
}

// MountSensitiveWithoutSystemd mocks base method.
func (m *MockMounter) MountSensitiveWithoutSystemd(source, target, fstype string, options, sensitiveOptions []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountSensitiveWithoutSystemd", source, target, fstype, options, sensitiveOptions)
	ret0, _ := ret[0].(error)
	return ret0
}

// MountSensitiveWithoutSystemd indicates an expected call of MountSensitiveWithoutSystemd.
func (mr *MockMounterMockRecorder) MountSensitiveWithoutSystemd(source, target, fstype, options, sensitiveOptions interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountSensitiveWithoutSystemd", reflect.TypeOf((*MockMounter)(nil).MountSensitiveWithoutSystemd), source, target, fstype, options, sensitiveOptions)
}

// MountSensitiveWithoutSystemdWithMountFlags mocks base method.
func (m *MockMounter) MountSensitiveWithoutSystemdWithMountFlags(source, target, fstype string, options, sensitiveOptions, mountFlags []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountSensitiveWithoutSystemdWithMountFlags", source, target, fstype, options, sensitiveOptions, mountFlags)
	ret0, _ := ret[0].(error)
	return ret0
}

// MountSensitiveWithoutSystemdWithMountFlags indicates an expected call of MountSensitiveWithoutSystemdWithMountFlags.
func (mr *MockMounterMockRecorder) MountSensitiveWithoutSystemdWithMountFlags(source, target, fstype, options, sensitiveOptions, mountFlags interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountSensitiveWithoutSystemdWithMountFlags", reflect.TypeOf((*MockMounter)(nil).MountSensitiveWithoutSystemdWithMountFlags), source, target, fstype, options, sensitiveOptions, mountFlags)
}

// NeedResize mocks base method.
func (m *MockMounter) NeedResize(devicePath, deviceMountPath string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NeedResize", devicePath, deviceMountPath)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NeedResize indicates an expected call of NeedResize.
func (mr *MockMounterMockRecorder) NeedResize(devicePath, deviceMountPath interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NeedResize", reflect.TypeOf((*MockMounter)(nil).NeedResize), devicePath, deviceMountPath)
}

// NewResizeFs mocks base method.
func (m *MockMounter) NewResizeFs() (Resizefs, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewResizeFs")
	ret0, _ := ret[0].(Resizefs)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewResizeFs indicates an expected call of NewResizeFs.
func (mr *MockMounterMockRecorder) NewResizeFs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewResizeFs", reflect.TypeOf((*MockMounter)(nil).NewResizeFs))
}

// RemoveStorageDevice mocks base method.
func (m *MockMounter) RemoveStorageDevice(dev string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveStorageDevice", dev)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveStorageDevice indicates an expected call of RemoveStorageDevice.
func (mr *MockMounterMockRecorder) RemoveStorageDevice(dev interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveStorageDevice", reflect.TypeOf((*MockMounter)(nil).RemoveStorageDevice), dev)
}

// RescanStorageDevice mocks base method.
func (m *MockMounter) RescanStorageDevice(dev string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RescanStorageDevice", dev)
	ret0, _ := ret[0].(error)
	return ret0
}

// RescanStorageDevice indicates an expected call of RescanStorageDevice.
func (mr *MockMounterMockRecorder) RescanStorageDevice(dev interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RescanStorageDevice", reflect.TypeOf((*MockMounter)(nil).RescanStorageDevice), dev)
}

// ScanStorageDevices mocks base method.
func (m *MockMounter) ScanStorageDevices() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ScanStorageDevices")
	ret0, _ := ret[0].(error)
	return ret0
}

// ScanStorageDevices indicates an expected call of ScanStorageDevices.
func (mr *MockMounterMockRecorder) ScanStorageDevices() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScanStorageDevices", reflect.TypeOf((*MockMounter)(nil).ScanStorageDevices))
}

// Unmount mocks base method.
func (m *MockMounter) Unmount(target string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unmount", target)
	ret0, _ := ret[0].(error)
	return ret0
}

// Unmount indicates an expected call of Unmount.
func (mr *MockMounterMockRecorder) Unmount(target interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unmount", reflect.TypeOf((*MockMounter)(nil).Unmount), target)
}

// MockResizefs is a mock of Resizefs interface.
type MockResizefs struct {
	ctrl     *gomock.Controller
	recorder *MockResizefsMockRecorder
}

// MockResizefsMockRecorder is the mock recorder for MockResizefs.
type MockResizefsMockRecorder struct {
	mock *MockResizefs
}

// NewMockResizefs creates a new mock instance.
func NewMockResizefs(ctrl *gomock.Controller) *MockResizefs {
	mock := &MockResizefs{ctrl: ctrl}
	mock.recorder = &MockResizefsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResizefs) EXPECT() *MockResizefsMockRecorder {
	return m.recorder
}

// Resize mocks base method.
func (m *MockResizefs) Resize(devicePath, deviceMountPath string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Resize", devicePath, deviceMountPath)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Resize indicates an expected call of Resize.
func (mr *MockResizefsMockRecorder) Resize(devicePath, deviceMountPath interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Resize", reflect.TypeOf((*MockResizefs)(nil).Resize), devicePath, deviceMountPath)
}
