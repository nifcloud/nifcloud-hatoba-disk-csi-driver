package driver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/nifcloud/nifcloud-hatoba-disk-csi-driver/pkg/cloud"
	"github.com/nifcloud/nifcloud-hatoba-disk-csi-driver/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultZone = "east-11"

	testClusterName = "test-cluster"
	testNodeName    = "test-node-01"
)

func assertStatusCodeEqual(t *testing.T, want codes.Code, gotErr error) {
	t.Helper()

	grpcErr, ok := status.FromError(gotErr)
	require.True(t, ok)
	assert.Equal(t, want, grpcErr.Code())
}

func TestNewControllerService(t *testing.T) {
	nrn := "nrn:nifcloud:hatoba:jp-east-1:USERID:cluster:test-cluster-id"

	tests := []struct {
		name              string
		env               map[string]string
		mockServerHandler http.Handler
		wantErr           bool
	}{
		{
			name: "success: create new controller service",
			env: map[string]string{
				"NIFCLOUD_HATOBA_CLUSTER_NRN": nrn,
				"NIFCLOUD_ACCESS_KEY_ID":      "dummy",
				"NIFCLOUD_SECRET_ACCESS_KEY":  "dummy",
			},
			mockServerHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"cluster": {"name": "%s"}}`, testClusterName)
			}),
		},
		{
			name: "error: NIFCLOUD_HATOBA_CLUSTER_NRN is empty",
			env: map[string]string{
				"NIFCLOUD_ACCESS_KEY_ID":     "dummy",
				"NIFCLOUD_SECRET_ACCESS_KEY": "dummy",
			},
			mockServerHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			wantErr:           true,
		},
		{
			name: "error: NIFCLOUD_HATOBA_CLUSTER_NRN is invalid",
			env: map[string]string{
				"NIFCLOUD_HATOBA_CLUSTER_NRN": "invalid",
				"NIFCLOUD_ACCESS_KEY_ID":      "dummy",
				"NIFCLOUD_SECRET_ACCESS_KEY":  "dummy",
			},
			mockServerHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler)
			defer ts.Close()

			for key, val := range tt.env {
				os.Setenv(key, val)
			}
			os.Setenv("NIFCLOUD_HATOBA_ENDPOINT", ts.URL)
			defer func() {
				for key := range tt.env {
					os.Unsetenv(key)
				}
				os.Unsetenv("NIFCLOUD_HATOBA_ENDPOINT")
			}()

			gotService, err := newControllerService()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NotNil(t, gotService)
			assert.NotNil(t, gotService.cloud)
			assert.NotNil(t, gotService.volumeLocks)
			assert.Equal(t, testClusterName, gotService.clusterName)
		})
	}
}

func TestCreateVolume(t *testing.T) {
	pvName := "test-volume"
	pvcName := "test-pvc"
	namespace := "default"
	volumeType := "standard-flash-a"
	volumeID := uuid.New().String()

	normalRequest := &csi.CreateVolumeRequest{
		Name:          pvName,
		CapacityRange: &csi.CapacityRange{RequiredBytes: int64(10 * util.GiB)},
		AccessibilityRequirements: &csi.TopologyRequirement{
			Requisite: []*csi.Topology{
				{
					Segments: map[string]string{
						TopologyKey: defaultZone,
					},
				},
			},
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		Parameters: map[string]string{
			parameterKeyPVCName:      pvcName,
			parameterKeyPVCNamespace: namespace,
			parameterKeyPVName:       pvName,
			VolumeTypeKey:            volumeType,
		},
	}
	invalidCapabilityRequest := &csi.CreateVolumeRequest{
		Name:                      normalRequest.Name,
		CapacityRange:             normalRequest.CapacityRange,
		AccessibilityRequirements: normalRequest.AccessibilityRequirements,
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
				},
			},
		},
		Parameters: normalRequest.Parameters,
	}
	invalidCapacityRangeRequest := &csi.CreateVolumeRequest{
		Name: normalRequest.Name,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: int64(2100 * util.GiB),
			LimitBytes:    int64(2000 * util.GiB),
		},
		AccessibilityRequirements: normalRequest.AccessibilityRequirements,
		VolumeCapabilities:        normalRequest.VolumeCapabilities,
		Parameters:                normalRequest.Parameters,
	}

	normalResponse := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeID,
			CapacityBytes: util.GiBToBytes(100),
			VolumeContext: map[string]string{},
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{
						TopologyKey:              defaultZone,
						wellKnownTopologyKeyZone: defaultZone,
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		cloud       func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud
		req         *csi.CreateVolumeRequest
		want        *csi.CreateVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: create volume",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					GetDiskByName(gomock.Any(), pvName, int64(10*util.GiB)).
					Return(nil, cloud.ErrNotFound)

				m.EXPECT().
					CreateDisk(gomock.Any(), testClusterName, &cloud.DiskOptions{
						VolumeType:    volumeType,
						CapacityBytes: int64(10 * util.GiB),
						Zone:          defaultZone,
						Tags: map[string]string{
							tagKeyCSIVolumeName:            pvName,
							tagKeyCreatedBy:                DriverName,
							tagKeyCreatedForClaimName:      pvcName,
							tagKeyCreatedForClaimNamespace: namespace,
							tagKeyCreatedForVolumeName:     pvName,
						},
					}).
					Return(&cloud.Volume{
						VolumeID:         volumeID,
						CapacityGiB:      100,
						AvailabilityZone: defaultZone,
					}, nil)

				return m
			},
			req:  normalRequest,
			want: normalResponse,
		},
		{
			name: "success: already exists same name and same capacity volume",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					GetDiskByName(gomock.Any(), pvName, int64(10*util.GiB)).
					Return(&cloud.Volume{
						VolumeID:         volumeID,
						CapacityGiB:      100,
						AvailabilityZone: defaultZone,
					}, nil)

				return m
			},
			req:  normalRequest,
			want: normalResponse,
		},
		{
			name: "error: same name volume exists but capacity is different",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					GetDiskByName(gomock.Any(), pvName, int64(10*util.GiB)).
					Return(nil, cloud.ErrDiskExistsDiffSize)

				return m
			},
			req:         normalRequest,
			wantErr:     true,
			wantErrCode: codes.AlreadyExists,
		},
		{
			name: "error: invalid capacity range",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				return cloud.NewMockCloud(ctrl)
			},
			req:         invalidCapacityRangeRequest,
			wantErr:     true,
			wantErrCode: codes.InvalidArgument,
		},
		{
			name: "error: invalid volume access mode",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				return cloud.NewMockCloud(ctrl)
			},
			req:         invalidCapabilityRequest,
			wantErr:     true,
			wantErrCode: codes.InvalidArgument,
		},
		{
			name: "error: create volume failed",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					GetDiskByName(gomock.Any(), pvName, int64(10*util.GiB)).
					Return(nil, cloud.ErrNotFound)

				m.EXPECT().
					CreateDisk(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("unknown error"))

				return m
			},
			req:         normalRequest,
			wantErr:     true,
			wantErrCode: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &controllerService{
				cloud:       tt.cloud(t, ctrl),
				volumeLocks: NewVolumeLocks(),
				clusterName: testClusterName,
			}
			got, err := d.CreateVolume(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDeleteVolume(t *testing.T) {
	volumeID := uuid.New().String()
	normalRequest := &csi.DeleteVolumeRequest{
		VolumeId: volumeID,
	}

	tests := []struct {
		name        string
		cloud       func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud
		req         *csi.DeleteVolumeRequest
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: delete volume",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					DeleteDisk(gomock.Any(), volumeID).
					Return(true, nil)

				return m
			},
			req: normalRequest,
		},
		{
			name: "success: already deleted",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					DeleteDisk(gomock.Any(), volumeID).
					Return(false, cloud.ErrNotFound)

				return m
			},
			req: normalRequest,
		},
		{
			name: "error: delete volume failed",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					DeleteDisk(gomock.Any(), volumeID).
					Return(false, fmt.Errorf("unknown error"))

				return m
			},
			req:         normalRequest,
			wantErr:     true,
			wantErrCode: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &controllerService{
				cloud:       tt.cloud(t, ctrl),
				volumeLocks: NewVolumeLocks(),
				clusterName: testClusterName,
			}
			got, err := d.DeleteVolume(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, &csi.DeleteVolumeResponse{}, got)
		})
	}
}

func TestControllerPublishVolume(t *testing.T) {
	volumeID := uuid.New().String()
	devicePath := "SCSI (0:1)"

	normalRequest := &csi.ControllerPublishVolumeRequest{
		VolumeId: volumeID,
		NodeId:   testNodeName,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	nonExistentNodeRequest := &csi.ControllerPublishVolumeRequest{
		VolumeId:         normalRequest.VolumeId,
		NodeId:           "non-existent",
		VolumeCapability: normalRequest.VolumeCapability,
	}
	nonExistentVolumeRequest := &csi.ControllerPublishVolumeRequest{
		VolumeId:         "non-existent",
		NodeId:           normalRequest.NodeId,
		VolumeCapability: normalRequest.VolumeCapability,
	}

	normalResponse := &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			devicePathKey: devicePath,
		},
	}

	tests := []struct {
		name        string
		cloud       func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud
		req         *csi.ControllerPublishVolumeRequest
		want        *csi.ControllerPublishVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: publish volume",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					IsExistNode(gomock.Any(), testClusterName, testNodeName).
					Return(true)

				m.EXPECT().
					GetDiskByID(gomock.Any(), volumeID).
					Return(&cloud.Volume{}, nil)

				m.EXPECT().
					AttachDisk(gomock.Any(), volumeID, testNodeName).
					Return(devicePath, nil)

				return m
			},
			req:  normalRequest,
			want: normalResponse,
		},
		{
			name: "error: node is not found",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					IsExistNode(gomock.Any(), testClusterName, nonExistentNodeRequest.NodeId).
					Return(false)

				return m
			},
			req:         nonExistentNodeRequest,
			wantErr:     true,
			wantErrCode: codes.NotFound,
		},
		{
			name: "error: volume is not found",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					IsExistNode(gomock.Any(), testClusterName, testNodeName).
					Return(true)

				m.EXPECT().
					GetDiskByID(gomock.Any(), nonExistentVolumeRequest.VolumeId).
					Return(nil, cloud.ErrNotFound)

				return m
			},
			req:         nonExistentVolumeRequest,
			wantErr:     true,
			wantErrCode: codes.NotFound,
		},
		{
			name: "error: attach disk failed",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					IsExistNode(gomock.Any(), testClusterName, testNodeName).
					Return(true)

				m.EXPECT().
					GetDiskByID(gomock.Any(), volumeID).
					Return(&cloud.Volume{}, nil)

				m.EXPECT().
					AttachDisk(gomock.Any(), volumeID, testNodeName).
					Return("", fmt.Errorf("unknown error"))

				return m
			},
			req:         normalRequest,
			wantErr:     true,
			wantErrCode: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &controllerService{
				cloud:       tt.cloud(t, ctrl),
				volumeLocks: NewVolumeLocks(),
				clusterName: testClusterName,
			}
			got, err := d.ControllerPublishVolume(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestControllerUnpublishVolume(t *testing.T) {
	volumeID := uuid.New().String()

	normalRequest := &csi.ControllerUnpublishVolumeRequest{
		VolumeId: volumeID,
		NodeId:   testNodeName,
	}

	tests := []struct {
		name        string
		cloud       func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud
		req         *csi.ControllerUnpublishVolumeRequest
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: unpublish volume",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					IsExistNode(gomock.Any(), testClusterName, testNodeName).
					Return(true)

				m.EXPECT().
					GetDiskByID(gomock.Any(), volumeID).
					Return(&cloud.Volume{}, nil)

				m.EXPECT().
					DetachDisk(gomock.Any(), volumeID, testNodeName).
					Return(nil)

				return m
			},
			req: normalRequest,
		},
		{
			name: "error: node is not found",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					IsExistNode(gomock.Any(), testClusterName, testNodeName).
					Return(false)

				return m
			},
			req:         normalRequest,
			wantErr:     true,
			wantErrCode: codes.NotFound,
		},
		{
			name: "error: disk is not found",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					IsExistNode(gomock.Any(), testClusterName, testNodeName).
					Return(true)

				m.EXPECT().
					GetDiskByID(gomock.Any(), volumeID).
					Return(nil, cloud.ErrNotFound)

				return m
			},
			req:         normalRequest,
			wantErr:     true,
			wantErrCode: codes.NotFound,
		},
		{
			name: "error: detach disk failed",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					IsExistNode(gomock.Any(), testClusterName, testNodeName).
					Return(true)

				m.EXPECT().
					GetDiskByID(gomock.Any(), volumeID).
					Return(&cloud.Volume{}, nil)

				m.EXPECT().
					DetachDisk(gomock.Any(), volumeID, testNodeName).
					Return(fmt.Errorf("unknown error"))

				return m
			},
			req:         normalRequest,
			wantErr:     true,
			wantErrCode: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &controllerService{
				cloud:       tt.cloud(t, ctrl),
				volumeLocks: NewVolumeLocks(),
				clusterName: testClusterName,
			}
			got, err := d.ControllerUnpublishVolume(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, &csi.ControllerUnpublishVolumeResponse{}, got)
		})
	}
}

func TestControllerExpandVolume(t *testing.T) {
	volumeID := uuid.New().String()

	baseRequest := &csi.ControllerExpandVolumeRequest{
		VolumeId: volumeID,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: int64(200 * util.GiB),
			LimitBytes:    int64(1000 * util.GiB),
		},
	}
	filesystemRequest := &csi.ControllerExpandVolumeRequest{
		VolumeId:      baseRequest.VolumeId,
		CapacityRange: baseRequest.CapacityRange,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
		},
	}
	blockDeviceRequest := &csi.ControllerExpandVolumeRequest{
		VolumeId:      baseRequest.VolumeId,
		CapacityRange: baseRequest.CapacityRange,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{
				Block: &csi.VolumeCapability_BlockVolume{},
			},
		},
	}

	tests := []struct {
		name        string
		cloud       func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud
		req         *csi.ControllerExpandVolumeRequest
		want        *csi.ControllerExpandVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: expand volume (filesystem)",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					ResizeDisk(gomock.Any(), volumeID, int64(200*util.GiB)).
					Return(int64(200), nil)

				return m
			},
			req: filesystemRequest,
			want: &csi.ControllerExpandVolumeResponse{
				CapacityBytes:         util.GiBToBytes(200),
				NodeExpansionRequired: true,
			},
		},
		{
			name: "success: expand volume (block)",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					ResizeDisk(gomock.Any(), volumeID, int64(200*util.GiB)).
					Return(int64(200), nil)

				return m
			},
			req: blockDeviceRequest,
			want: &csi.ControllerExpandVolumeResponse{
				CapacityBytes:         util.GiBToBytes(200),
				NodeExpansionRequired: false,
			},
		},
		{
			name: "error: requested size is over the max size",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				return cloud.NewMockCloud(ctrl)
			},
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: baseRequest.VolumeId,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: int64(200 * util.GiB),
					LimitBytes:    int64(100 * util.GiB),
				},
				VolumeCapability: filesystemRequest.VolumeCapability,
			},
			wantErr:     true,
			wantErrCode: codes.InvalidArgument,
		},
		{
			name: "error: resize disk failed",
			cloud: func(t *testing.T, ctrl *gomock.Controller) cloud.Cloud {
				m := cloud.NewMockCloud(ctrl)

				m.EXPECT().
					ResizeDisk(gomock.Any(), volumeID, int64(200*util.GiB)).
					Return(int64(0), fmt.Errorf("unknown error"))

				return m
			},
			req:         filesystemRequest,
			wantErr:     true,
			wantErrCode: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &controllerService{
				cloud:       tt.cloud(t, ctrl),
				volumeLocks: NewVolumeLocks(),
				clusterName: testClusterName,
			}
			got, err := d.ControllerExpandVolume(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
