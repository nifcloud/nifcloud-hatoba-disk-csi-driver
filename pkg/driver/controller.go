package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/nifcloud/nifcloud-hatoba-disk-csi-driver/pkg/cloud"
	"github.com/nifcloud/nifcloud-hatoba-disk-csi-driver/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

var (
	volumeCaps = []csi.VolumeCapability_AccessMode{
		{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}

	controllerCaps = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	}
)

type controllerService struct {
	cloud       cloud.Cloud
	volumeLocks *VolumeLocks
	clusterName string
}

func newControllerService() (service controllerService, err error) {
	service.volumeLocks = NewVolumeLocks()

	clusterNRN := os.Getenv("NIFCLOUD_HATOBA_CLUSTER_NRN")
	if clusterNRN == "" {
		err = fmt.Errorf("NIFCLOUD_HATOBA_CLUSTER_NRN is required")
		return
	}

	region, err := detectClusterRegion(clusterNRN)
	if err != nil {
		return
	}
	klog.Infof("detected region: %s", region)

	cloud, err := cloud.NewCloud(region)
	if err != nil {
		return
	}
	service.cloud = cloud

	clusterName, err := cloud.GetClusterName(context.Background(), clusterNRN)
	if err != nil {
		return
	}
	klog.Infof("detected cluster name: %s", clusterName)
	service.clusterName = clusterName

	return
}

func detectClusterRegion(nrn string) (string, error) {
	// NRN format: https://pfs.nifcloud.com/spec/common/nrn.htm
	parts := strings.Split(nrn, ":")
	if len(parts) != 7 {
		return "", fmt.Errorf("the format of NRN is not valid")
	}

	return parts[3], nil
}

func (d *controllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(4).Infof("CreateVolume: called with args %+v", *req)
	volName := req.GetName()
	if len(volName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name not provided")
	}

	volSizeBytes, err := getVolSizeBytes(req)
	if err != nil {
		return nil, err
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	if !isValidVolumeCapabilities(volCaps) {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not supported")
	}

	if acquire := d.volumeLocks.TryAcquire(volName); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volName)
	}
	defer d.volumeLocks.Release(volName)

	disk, err := d.cloud.GetDiskByName(ctx, volName, volSizeBytes)
	if err != nil {
		switch err {
		case cloud.ErrNotFound:
		case cloud.ErrMultiDisks:
			return nil, status.Error(codes.Internal, err.Error())
		case cloud.ErrDiskExistsDiffSize:
			return nil, status.Error(codes.AlreadyExists, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// volume already exists
	if disk != nil {
		return newCreateVolumeResponse(disk), nil
	}

	var (
		volumeType string
		volumeTags = map[string]string{
			cloud.TagKeyCSIVolumeName: volName,
		}
	)
	for key, value := range req.GetParameters() {
		switch strings.ToLower(key) {
		case "fstype":
			klog.Warning("\"fstype\" is deprecated, please use \"csi.storage.k8s.io/fstype\" instead")
		case VolumeTypeKey:
			volumeType = value
		case parameterKeyPVCName:
			volumeTags[tagKeyCreatedForClaimName] = value
		case parameterKeyPVCNamespace:
			volumeTags[tagKeyCreatedForClaimNamespace] = value
		case parameterKeyPVName:
			volumeTags[tagKeyCreatedForVolumeName] = value
		default:
			return nil, status.Errorf(codes.InvalidArgument, "Invalid parameter key %s for CreateVolume", key)
		}
	}
	if len(volumeTags) > 0 {
		volumeTags[tagKeyCreatedBy] = DriverName
	}

	zone := pickAvailabilityZone(req.GetAccessibilityRequirements())
	if zone == "" {
		return nil, status.Errorf(codes.Internal, "Could not detect the zone to create disk")
	}
	klog.V(1).Infof("create volume in %s zone", zone)

	// create a new volume
	opts := &cloud.DiskOptions{
		CapacityBytes: volSizeBytes,
		VolumeType:    volumeType,
		Zone:          zone,
		Tags:          volumeTags,
	}

	disk, err = d.cloud.CreateDisk(ctx, d.clusterName, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create volume %q: %v", volName, err)
	}

	return newCreateVolumeResponse(disk), nil
}

func (d *controllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).Infof("DeleteVolume: called with args: %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	if acquire := d.volumeLocks.TryAcquire(volumeID); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volumeID)
	}
	defer d.volumeLocks.Release(volumeID)

	if _, err := d.cloud.DeleteDisk(ctx, volumeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			klog.V(4).Info("DeleteVolume: volume not found, returning with success")
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Could not delete volume ID %q: %v", volumeID, err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (d *controllerService) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.V(4).Infof("ControllerPublishVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	nodeID := req.GetNodeId()
	if len(nodeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Node ID not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	caps := []*csi.VolumeCapability{volCap}
	if !isValidVolumeCapabilities(caps) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	if !d.cloud.IsExistNode(ctx, d.clusterName, nodeID) {
		return nil, status.Errorf(codes.NotFound, "Instance %q not found", nodeID)
	}

	if _, err := d.cloud.GetDiskByID(ctx, volumeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "Volume not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	if acquire := d.volumeLocks.TryAcquire(volumeID); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volumeID)
	}
	defer d.volumeLocks.Release(volumeID)

	devicePath, err := d.cloud.AttachDisk(ctx, volumeID, nodeID)
	if err != nil {
		if errors.Is(err, cloud.ErrAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "Could not attach volume %q to node %q: %v", volumeID, nodeID, err)
	}
	klog.V(5).Infof("ControllerPublishVolume: volume %s attached to node %s through device %s", volumeID, nodeID, devicePath)

	pvInfo := map[string]string{devicePathKey: devicePath}
	return &csi.ControllerPublishVolumeResponse{PublishContext: pvInfo}, nil
}

func (d *controllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.V(4).Infof("ControllerUnpublishVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	nodeID := req.GetNodeId()
	if len(nodeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Node ID not provided")
	}

	if !d.cloud.IsExistNode(ctx, d.clusterName, nodeID) {
		return nil, status.Errorf(codes.NotFound, "Node %q not found", nodeID)
	}

	if _, err := d.cloud.GetDiskByID(ctx, volumeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "Volume %q not found", volumeID)
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	if acquire := d.volumeLocks.TryAcquire(volumeID); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volumeID)
	}
	defer d.volumeLocks.Release(volumeID)

	if err := d.cloud.DetachDisk(ctx, volumeID, nodeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			klog.Warningf("treating volume %s as unpublished because resource could not be found: %v", volumeID, err)
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Could not detach volume %q from node %q: %v", volumeID, nodeID, err)
	}
	klog.V(5).Infof("ControllerUnpublishVolume: volume %s detached from node %s", volumeID, nodeID)

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *controllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Infof("ControllerGetCapabilities: called with args %+v", *req)
	var caps []*csi.ControllerServiceCapability
	for _, cap := range controllerCaps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (d *controllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).Infof("GetCapacity: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *controllerService) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).Infof("ListVolumes: called with args %+v", *req)
	return nil, status.Errorf(codes.Unimplemented, "")
}

func (d *controllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	if _, err := d.cloud.GetDiskByID(ctx, volumeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "Volume not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(volCaps) {
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func (d *controllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(4).Infof("ControllerExpandVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range not provided")
	}

	newSize := util.RoundUpBytes(capRange.GetRequiredBytes())
	maxVolSize := capRange.GetLimitBytes()
	if maxVolSize > 0 && maxVolSize < newSize {
		return nil, status.Error(codes.InvalidArgument, "After round-up, volume size exceeds the limit specified")
	}

	actualSizeGiB, err := d.cloud.ResizeDisk(ctx, volumeID, newSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not resize volume %q: %v", volumeID, err)
	}

	nodeExpansionRequired := true
	// if this is a raw block device, no expansion should be necessary on the node
	cap := req.GetVolumeCapability()
	if cap != nil && cap.GetBlock() != nil {
		nodeExpansionRequired = false
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         util.GiBToBytes(actualSizeGiB),
		NodeExpansionRequired: nodeExpansionRequired,
	}, nil
}

func (d *controllerService) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(4).Infof("ControllerGetVolume: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *controllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).Infof("CreateSnapshot: called with args %+v", req)
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot is not implemented (NIFCLOUD does not support the snapshot for volume)")
}

func (d *controllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(4).Infof("DeleteSnapshot: called with args %+v", req)
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot is not implemented (NIFCLOUD does not support the snapshot for volume)")
}

func (d *controllerService) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(4).Infof("ListSnapshots: called with args %+v", req)
	return nil, status.Error(codes.Unimplemented, "ListSnapshots is not implemented (NIFCLOUD does not support the snapshot for volume)")
}

// pickAvailabilityZone selects 1 zone given topology requirement.
// if not found, empty string is returned.
func pickAvailabilityZone(requirement *csi.TopologyRequirement) string {
	if requirement == nil {
		return ""
	}
	for _, topology := range requirement.GetPreferred() {
		zone, exists := topology.GetSegments()[wellKnownTopologyKeyZone]
		if exists {
			return zone
		}

		zone, exists = topology.GetSegments()[TopologyKey]
		if exists {
			return zone
		}
	}
	for _, topology := range requirement.GetRequisite() {
		zone, exists := topology.GetSegments()[wellKnownTopologyKeyZone]
		if exists {
			return zone
		}

		zone, exists = topology.GetSegments()[TopologyKey]
		if exists {
			return zone
		}
	}
	return ""
}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	hasSupport := func(cap *csi.VolumeCapability) bool {
		for _, c := range volumeCaps {
			if c.GetMode() == cap.AccessMode.GetMode() {
				return true
			}
		}
		return false
	}

	foundAll := true
	for _, c := range volCaps {
		if !hasSupport(c) {
			foundAll = false
		}
	}
	return foundAll
}

func newCreateVolumeResponse(disk *cloud.Volume) *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      disk.VolumeID,
			CapacityBytes: util.GiBToBytes(disk.CapacityGiB),
			VolumeContext: map[string]string{},
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{TopologyKey: disk.AvailabilityZone, wellKnownTopologyKeyZone: disk.AvailabilityZone},
				},
			},
		},
	}
}

func getVolSizeBytes(req *csi.CreateVolumeRequest) (int64, error) {
	var volSizeBytes int64
	capRange := req.GetCapacityRange()
	if capRange == nil {
		volSizeBytes = cloud.DefaultVolumeSize
	} else {
		volSizeBytes = util.RoundUpBytes(capRange.GetRequiredBytes())
		maxVolSize := capRange.GetLimitBytes()
		if maxVolSize > 0 && maxVolSize < volSizeBytes {
			return 0, status.Error(codes.InvalidArgument, "After round-up, volume size exceeds the limit specified")
		}
	}
	return volSizeBytes, nil
}
