package driver

import (
	"context"
	"os"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

const (
	testScsiID          = "SCSI (0:1)"
	testDevicePath      = "/dev/sdb"
	testGlobalMountPath = "/mnt/test"
	testStagingPath     = "/mnt/test/staging"
	testMountPath       = "/mnt/test/mount"
	testVolumeID        = "test-volume"
)

func TestNodeStageVolume(t *testing.T) {
	tests := []struct {
		name        string
		mounter     func(ctrl *gomock.Controller) Mounter
		req         *csi.NodeStageVolumeRequest
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: stage volume (filesystem)",
			mounter: func(ctrl *gomock.Controller) Mounter {
				m := NewMockMounter(ctrl)

				m.EXPECT().ScanStorageDevices().Return(nil)
				m.EXPECT().FindDevicePath(testScsiID).Return(testDevicePath, nil)
				m.EXPECT().ExistsPath(testStagingPath).Return(false, nil)
				m.EXPECT().MakeDir(testStagingPath).Return(nil)
				m.EXPECT().GetDeviceName(testStagingPath).Return("", 1, nil)
				m.EXPECT().NeedResize(testDevicePath, testStagingPath)
				m.EXPECT().FormatAndMount(testDevicePath, testStagingPath, FSTypeExt4, nil)

				return m
			},
			req: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{devicePathKey: testScsiID},
				StagingTargetPath: testStagingPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: FSTypeExt4,
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: testVolumeID,
			},
		},
		{
			name: "success: stage volume (block)",
			mounter: func(ctrl *gomock.Controller) Mounter {
				m := NewMockMounter(ctrl)

				m.EXPECT().ScanStorageDevices().Return(nil)

				return m
			},
			req: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{devicePathKey: testScsiID},
				StagingTargetPath: testStagingPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: testVolumeID,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			n := &nodeService{
				nodeName:    testNodeName,
				mounter:     tt.mounter(ctrl),
				volumeLocks: NewVolumeLocks(),
			}
			got, err := n.NodeStageVolume(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, &csi.NodeStageVolumeResponse{}, got)
		})
	}
}

func TestNodeUnstageVolume(t *testing.T) {
	tests := []struct {
		name        string
		mounter     func(ctrl *gomock.Controller) Mounter
		req         *csi.NodeUnstageVolumeRequest
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: unstage volume",
			mounter: func(ctrl *gomock.Controller) Mounter {
				m := NewMockMounter(ctrl)

				m.EXPECT().GetDeviceName(testStagingPath).Return(testDevicePath, 1, nil)
				m.EXPECT().Unmount(testStagingPath).Return(nil)
				m.EXPECT().RemoveStorageDevice(testDevicePath).Return(nil)

				return m
			},
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: testStagingPath,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			n := &nodeService{
				nodeName:    testNodeName,
				mounter:     tt.mounter(ctrl),
				volumeLocks: NewVolumeLocks(),
			}
			got, err := n.NodeUnstageVolume(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, &csi.NodeUnstageVolumeResponse{}, got)
		})
	}
}

func TestNodePublishVolume(t *testing.T) {
	tests := []struct {
		name        string
		mounter     func(ctrl *gomock.Controller) Mounter
		req         *csi.NodePublishVolumeRequest
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: publish volume (filesystem)",
			mounter: func(ctrl *gomock.Controller) Mounter {
				m := NewMockMounter(ctrl)

				m.EXPECT().MakeDir(testMountPath).Return(nil)
				m.EXPECT().IsLikelyNotMountPoint(testMountPath).Return(true, nil)
				m.EXPECT().Mount(testStagingPath, testMountPath, FSTypeExt4, []string{"bind"}).Return(nil)

				return m
			},
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: testStagingPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: FSTypeExt4,
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				TargetPath: testMountPath,
			},
		},
		{
			name: "success: publish volume (block)",
			mounter: func(ctrl *gomock.Controller) Mounter {
				m := NewMockMounter(ctrl)

				m.EXPECT().FindDevicePath(testScsiID).Return(testDevicePath, nil)
				m.EXPECT().ExistsPath(testGlobalMountPath).Return(false, nil)
				m.EXPECT().MakeDir(testGlobalMountPath).Return(nil)
				m.EXPECT().MakeFile(testMountPath).Return(nil)
				m.EXPECT().IsLikelyNotMountPoint(testMountPath).Return(true, nil)
				m.EXPECT().Mount(testDevicePath, testMountPath, "", []string{"bind"}).Return(nil)

				return m
			},
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: testStagingPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				TargetPath: testMountPath,
				PublishContext: map[string]string{
					devicePathKey: testScsiID,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			n := &nodeService{
				nodeName:    testNodeName,
				mounter:     tt.mounter(ctrl),
				volumeLocks: NewVolumeLocks(),
			}
			got, err := n.NodePublishVolume(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, &csi.NodePublishVolumeResponse{}, got)
		})
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	tests := []struct {
		name        string
		mounter     func(ctrl *gomock.Controller) Mounter
		req         *csi.NodeUnpublishVolumeRequest
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: unpublish volume",
			mounter: func(ctrl *gomock.Controller) Mounter {
				m := NewMockMounter(ctrl)

				m.EXPECT().Unmount(testMountPath).Return(nil)

				return m
			},
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   testVolumeID,
				TargetPath: testMountPath,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			n := &nodeService{
				nodeName:    testNodeName,
				mounter:     tt.mounter(ctrl),
				volumeLocks: NewVolumeLocks(),
			}
			got, err := n.NodeUnpublishVolume(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, &csi.NodeUnpublishVolumeResponse{}, got)
		})
	}
}

func TestNodeGetVolumeStats(t *testing.T) {
	targetPath, err := os.MkdirTemp("", "nifcloud-hatoba-disk-csi-driver")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(targetPath); err != nil {
			t.Log(err)
		}
	}()

	tests := []struct {
		name        string
		mounter     func(ctrl *gomock.Controller) Mounter
		req         *csi.NodeGetVolumeStatsRequest
		want        *csi.NodeGetVolumeStatsResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			name: "success: get stats",
			mounter: func(ctrl *gomock.Controller) Mounter {
				m := NewMockMounter(ctrl)

				m.EXPECT().ExistsPath(targetPath).Return(true, nil)

				return m
			},
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   testVolumeID,
				VolumePath: targetPath,
			},
			want: &csi.NodeGetVolumeStatsResponse{
				Usage: []*csi.VolumeUsage{
					{Unit: csi.VolumeUsage_BYTES},
					{Unit: csi.VolumeUsage_INODES},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			n := &nodeService{
				nodeName:    testNodeName,
				mounter:     tt.mounter(ctrl),
				volumeLocks: NewVolumeLocks(),
			}
			got, err := n.NodeGetVolumeStats(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assertStatusCodeEqual(t, tt.wantErrCode, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Len(t, got.Usage, len(tt.want.Usage))
			for i, u := range got.Usage {
				assert.Equal(t, tt.want.Usage[i].Unit, u.Unit)
				assert.NotEmpty(t, u.Available)
				assert.NotEmpty(t, u.Total)
				assert.NotEmpty(t, u.Used)
			}
		})
	}
}
