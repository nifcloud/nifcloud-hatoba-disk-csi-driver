package driver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

const (
	// FSTypeExt2 represents the ext2 filesystem type
	FSTypeExt2 = "ext2"
	// FSTypeExt3 represents the ext3 filesystem type
	FSTypeExt3 = "ext3"
	// FSTypeExt4 represents the ext4 filesystem type
	FSTypeExt4 = "ext4"
	// FSTypeXfs represents te xfs filesystem type
	FSTypeXfs = "xfs"

	// default file system type to be used when it is not provided
	defaultFsType = FSTypeExt4

	// maxVolumesPerNode is the maximum number of volumes that an NIFCLOUD instance can have attached.
	// More info at https://pfs.nifcloud.com/service/disk.htm
	maxVolumesPerNode = 14

	failureDomainZoneLabel = "failure-domain.beta.kubernetes.io/zone"
)

var (
	// ValidFSTypes is valid filesystem type.
	ValidFSTypes = map[string]struct{}{
		FSTypeExt2: {},
		FSTypeExt3: {},
		FSTypeExt4: {},
		FSTypeXfs:  {},
	}
)

var (
	// nodeCaps represents the capability of node service.
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
	}
)

// nodeService represents the node service of CSI driver
type nodeService struct {
	nodeName    string
	mounter     Mounter
	volumeLocks *VolumeLocks
}

// newNodeService creates a new node service
// it panics if failed to create the service
func newNodeService(nodeName string) nodeService {
	return nodeService{
		nodeName:    nodeName,
		mounter:     newNodeMounter(),
		volumeLocks: NewVolumeLocks(),
	}
}

func (n *nodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(4).Infof("NodeStageVolume: called with args: %+v", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Errorf(codes.InvalidArgument, "Volume capability not supported: %v", volCap)
	}

	if err := n.mounter.ScanStorageDevices(); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not scan the SCSI storages: %v", err)
	}

	// If the access type is block, do nothing for stage.
	switch volCap.GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
		return &csi.NodeStageVolumeResponse{}, nil
	}

	mount := volCap.GetMount()
	if mount == nil {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume: mount is nil within volume capability")
	}
	fsType := mount.GetFsType()
	if len(fsType) == 0 {
		fsType = defaultFsType
	}

	if acquire := n.volumeLocks.TryAcquire(volumeID); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volumeID)
	}
	defer n.volumeLocks.Release(volumeID)

	devicePath, ok := req.PublishContext[devicePathKey]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "Device path not provided")
	}

	source, err := n.mounter.FindDevicePath(devicePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to find device path %s: %v", devicePath, err)
	}

	klog.V(4).Infof("NodeStageVolume: find device path %s -> %s", devicePath, source)

	exists, err := n.mounter.ExistsPath(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if target %q exists: %v", target, err)
	}

	if !exists {
		klog.V(4).Infof("NodeStageVolume: creating target dir %q", target)
		if err = n.mounter.MakeDir(target); err != nil {
			return nil, status.Errorf(codes.Internal, "could not create target dir %q: %v", target, err)
		}
	}

	device, _, err := n.mounter.GetDeviceName(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if volume is already mounted: %v", err)
	}

	if device == source {
		klog.V(4).Infof("NodeStageVolume: volume=%q already staged", volumeID)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	klog.V(5).Infof("NodeStageVolume: formatting %s and mounting at %s with fstype %s", source, target, fsType)
	if err := n.mounter.FormatAndMount(source, target, fsType, nil); err != nil {
		return nil, status.Errorf(codes.Internal, "could not format %q and mount it at %q", source, target)
	}

	needResize, err := n.mounter.NeedResize(source, target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not determine if volume %q (%q) need to be resized:  %v", req.GetVolumeId(), source, err)
	}

	if needResize {
		r, err := n.mounter.NewResizeFs()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Error attempting to create new ResizeFs:  %v", err)
		}

		klog.V(2).Infof("Volume %s needs resizing", source)
		if _, err := r.Resize(source, target); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not resize volume %q (%q):  %v", volumeID, source, err)
		}
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (n *nodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(4).Infof("NodeUnstageVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	if acquire := n.volumeLocks.TryAcquire(volumeID); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volumeID)
	}
	defer n.volumeLocks.Release(volumeID)

	dev, refCount, err := n.mounter.GetDeviceName(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if volume is mounted: %v", err)
	}

	if refCount == 0 {
		klog.V(5).Infof("NodeUnstageVolume: %s target not mounted", target)
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	if refCount > 1 {
		klog.Warningf("NodeUnstageVolume: found %d references to device %s mounted at target path %s", refCount, dev, target)
	}

	klog.V(5).Infof("NodeUnstageVolume: unmounting %s", target)
	if err = n.mounter.Unmount(target); err != nil {
		if isNotMountedError(err) {
			klog.V(4).Infof("NodeUnstageVolume: %s is not mounted", target)
		} else {
			return nil, status.Errorf(codes.Internal, "Could not unmount target %q: %v", target, err)
		}
	}

	klog.Infof("NodeUnstageVolume: removing storage device of %q", dev)
	if err := n.mounter.RemoveStorageDevice(dev); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not remove the storage device: %v", err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil

}

func (n *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	klog.V(4).Infof("NodeExpandVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volumePath := req.GetVolumePath()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume path must be provided")
	}

	if acquire := n.volumeLocks.TryAcquire(volumeID); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volumeID)
	}
	defer n.volumeLocks.Release(volumeID)

	volumeCapability := req.GetVolumeCapability()
	if volumeCapability != nil {
		caps := []*csi.VolumeCapability{volumeCapability}
		if !isValidVolumeCapabilities(caps) {
			return nil, status.Errorf(codes.InvalidArgument, "VolumeCapability is invalid: %v", volumeCapability)
		}

		if blk := volumeCapability.GetBlock(); blk != nil {
			klog.V(4).Infof("NodeExpandVolume called for %v at %s. Since it is a block device, ignoring...", volumeID, volumePath)
			return &csi.NodeExpandVolumeResponse{}, nil
		}
	} else {
		isBlock, err := n.isBlockDevice(volumePath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to determine if volumePath [%v] is a block device: %v", volumePath, err)
		}

		if isBlock {
			bcap, err := n.getBlockSizeBytes(volumePath)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to get block capacity on path %s: %v", volumePath, err)
			}
			klog.V(4).Infof("NodeExpandVolume called for %v at %s. Since it is a block device, ignoring...", volumeID, volumePath)
			return &csi.NodeExpandVolumeResponse{CapacityBytes: bcap}, nil
		}
	}

	device, _, err := n.mounter.GetDeviceName(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get device name from mount %s: %v", volumePath, err)
	}

	if err := n.mounter.RescanStorageDevice(device); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not rescan the storage device %s: %v", device, err)
	}

	r, err := n.mounter.NewResizeFs()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error attempting to create new ResizeFs: %v", err)
	}

	if _, err := r.Resize(device, volumePath); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not resize volume %q (%q):  %v", volumeID, device, err)
	}

	bcap, err := n.getBlockSizeBytes(device)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get block capacity on path %s: %v", req.VolumePath, err)
	}

	return &csi.NodeExpandVolumeResponse{CapacityBytes: bcap}, nil
}

func (n *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.V(4).Infof("NodePublishVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	source := req.GetStagingTargetPath()
	if len(source) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Errorf(codes.InvalidArgument, "Volume capability not supported: %v", volCap)
	}

	if acquire := n.volumeLocks.TryAcquire(volumeID); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volumeID)
	}
	defer n.volumeLocks.Release(volumeID)

	mountOptions := []string{"bind"}
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	switch mode := volCap.GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
		if err := n.nodePublishVolumeForBlock(req, mountOptions); err != nil {
			return nil, err
		}
	case *csi.VolumeCapability_Mount:
		if err := n.nodePublishVolumeForFileSystem(req, mountOptions, mode); err != nil {
			return nil, err
		}
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (n *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(4).Infof("NodeUnpublishVolume: called with args %+v", *req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	if acquire := n.volumeLocks.TryAcquire(volumeID); !acquire {
		return nil, status.Errorf(codes.Aborted, "The operation for volume id %q is now in progress", volumeID)
	}
	defer n.volumeLocks.Release(volumeID)

	klog.V(4).Infof("NodeUnpublishVolume: unmounting %s", target)
	if err := n.mounter.Unmount(target); err != nil {
		if isNotMountedError(err) {
			klog.Infof("NodeUnpublishVolume: treating target %s as unpublished because it is not mounted.", target)
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Could not unmount %q: %v", target, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (n *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(4).Infof("NodeGetVolumeStats: called with args %+v", *req)
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume ID was empty")
	}
	if len(req.VolumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume path was empty")
	}

	exists, err := n.mounter.ExistsPath(req.VolumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unknown error when stat on %s: %v", req.VolumePath, err)
	}
	if !exists {
		return nil, status.Errorf(codes.NotFound, "path %s does not exist", req.VolumePath)
	}

	isBlock, err := n.isBlockDevice(req.VolumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to determine whether %s is block device: %v", req.VolumePath, err)
	}

	if isBlock {
		bcap, err := n.getBlockSizeBytes(req.VolumePath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get block capacity on path %s: %v", req.VolumePath, err)
		}

		return &csi.NodeGetVolumeStatsResponse{
			Usage: []*csi.VolumeUsage{
				{
					Unit:  csi.VolumeUsage_BYTES,
					Total: bcap,
				},
			},
		}, nil
	}

	available, capacity, used, inodes, inodesFree, inodesUsed, err := getFsInfo(req.VolumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get FsInfo due to error: %v", err)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Unit:      csi.VolumeUsage_BYTES,
				Available: resource.NewQuantity(available, resource.BinarySI).AsDec().UnscaledBig().Int64(),
				Total:     resource.NewQuantity(capacity, resource.BinarySI).AsDec().UnscaledBig().Int64(),
				Used:      resource.NewQuantity(used, resource.BinarySI).AsDec().UnscaledBig().Int64(),
			},
			{
				Unit:      csi.VolumeUsage_INODES,
				Available: resource.NewQuantity(inodesFree, resource.BinarySI).AsDec().UnscaledBig().Int64(),
				Total:     resource.NewQuantity(inodes, resource.BinarySI).AsDec().UnscaledBig().Int64(),
				Used:      resource.NewQuantity(inodesUsed, resource.BinarySI).AsDec().UnscaledBig().Int64(),
			},
		},
	}, nil
}

func (n *nodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Infof("NodeGetCapabilities: called with args %+v", *req)
	var caps []*csi.NodeServiceCapability
	for _, cap := range nodeCaps {
		c := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (n *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("NodeGetInfo: called with args %+v", *req)

	zone, err := n.detectAvailabilityZoneOfNode()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get node availability zone: %s", err)
	}

	klog.Infof("detected node placed zone: %s", zone)

	topology := &csi.Topology{
		Segments: map[string]string{TopologyKey: zone, wellKnownTopologyKeyZone: zone},
	}

	return &csi.NodeGetInfoResponse{
		NodeId:             n.nodeName,
		MaxVolumesPerNode:  maxVolumesPerNode,
		AccessibleTopology: topology,
	}, nil
}

func (n *nodeService) nodePublishVolumeForBlock(req *csi.NodePublishVolumeRequest, mountOptions []string) error {
	target := req.GetTargetPath()

	devicePath, exists := req.PublishContext[devicePathKey]
	if !exists {
		return status.Error(codes.InvalidArgument, "Device path not provided")
	}
	source, err := n.mounter.FindDevicePath(devicePath)
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to find device path %s. %v", devicePath, err)
	}

	klog.V(4).Infof("NodePublishVolume [block]: find device path %s -> %s", devicePath, source)

	globalMountPath := filepath.Dir(target)

	// create the global mount path if it is missing
	// Path in the form of /var/lib/kubelet/plugins/kubernetes.io/csi/volumeDevices/publish/{volumeName}
	exists, err = n.mounter.ExistsPath(globalMountPath)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not check if path exists %q: %v", globalMountPath, err)
	}

	if !exists {
		if err := n.mounter.MakeDir(globalMountPath); err != nil {
			return status.Errorf(codes.Internal, "Could not create dir %q: %v", globalMountPath, err)
		}
	}

	// Create the mount point as a file since bind mount device node requires it to be a file
	klog.V(4).Infof("NodePublishVolume [block]: making target file %s", target)
	if err = n.mounter.MakeFile(target); err != nil {
		if removeErr := os.Remove(target); removeErr != nil {
			return status.Errorf(codes.Internal, "Could not remove mount target %q: %v", target, removeErr)
		}
		return status.Errorf(codes.Internal, "Could not create file %q: %v", target, err)
	}

	// Checking if the target file is already mounted with a device.
	mounted, err := n.isMounted(source, target)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not check if %q is mounted: %v", target, err)
	}

	if mounted {
		klog.V(4).Infof("NodePublishVolume [block]: Target path %q is already mounted", target)
		return nil
	}

	klog.V(4).Infof("NodePublishVolume [block]: mounting %s at %s", source, target)
	if err := n.mounter.Mount(source, target, "", mountOptions); err != nil {
		if removeErr := os.Remove(target); removeErr != nil {
			return status.Errorf(codes.Internal, "Could not remove mount target %q: %v", target, removeErr)
		}
		return status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, target, err)
	}

	return nil
}

func (n *nodeService) nodePublishVolumeForFileSystem(req *csi.NodePublishVolumeRequest, mountOptions []string, mode *csi.VolumeCapability_Mount) error {
	target := req.GetTargetPath()
	source := req.GetStagingTargetPath()
	if m := mode.Mount; m != nil {
		hasOption := func(options []string, opt string) bool {
			for _, o := range options {
				if o == opt {
					return true
				}
			}
			return false
		}
		for _, f := range m.MountFlags {
			if !hasOption(mountOptions, f) {
				mountOptions = append(mountOptions, f)
			}
		}
	}

	klog.V(4).Infof("NodePublishVolume: creating dir %s", target)
	if err := n.mounter.MakeDir(target); err != nil {
		return status.Errorf(codes.Internal, "Could not create dir %q: %v", target, err)
	}

	// Checking if the target directory is already mounted with a device.
	mounted, err := n.isMounted(source, target)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not check if %q is mounted: %v", target, err)
	}

	if mounted {
		klog.V(4).Infof("NodePublishVolume: Target path %q is already mounted", target)
		return nil
	}

	fsType := mode.Mount.GetFsType()
	if len(fsType) == 0 {
		fsType = defaultFsType
	}

	if _, ok := ValidFSTypes[strings.ToLower(fsType)]; !ok {
		return status.Errorf(codes.InvalidArgument, "NodePublishVolume: invalid fstype %s", fsType)
	}

	klog.V(4).Infof("NodePublishVolume: mounting %s at %s with option %s as fstype %s", source, target, mountOptions, fsType)
	if err := n.mounter.Mount(source, target, fsType, mountOptions); err != nil {
		return status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, target, err)
	}

	return nil
}

// isMounted checks if target is mounted. It does NOT return an error if target doesn't exist.
func (n *nodeService) isMounted(source string, target string) (bool, error) {
	notMnt, err := n.mounter.IsLikelyNotMountPoint(target)
	if err != nil && !os.IsNotExist(err) {
		// Checking if the path exists and error is related to Corrupted Mount, in that case, the system could unmount and mount.
		_, pathErr := n.mounter.ExistsPath(target)
		if pathErr != nil && n.mounter.IsCorruptedMnt(pathErr) {
			klog.V(4).Infof("NodePublishVolume: Target path %q is a corrupted mount. Trying to unmount.", target)
			if mntErr := n.mounter.Unmount(target); mntErr != nil {
				return false, status.Errorf(codes.Internal, "Unable to unmount the target %q : %v", target, mntErr)
			}
			return false, nil
		}
		return false, status.Errorf(codes.Internal, "Could not check if %q is a mount point: %v, %v", target, err, pathErr)
	}

	if err != nil && os.IsNotExist(err) {
		klog.V(4).Infof("NodePublishVolume: Target path %q does not exist", target)
		return false, nil
	}

	if !notMnt {
		klog.V(4).Infof("NodePublishVolume: Target path %q is already mounted", target)
	}

	return !notMnt, nil
}

func (n *nodeService) detectAvailabilityZoneOfNode() (string, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("cannot create Kubernetes client config: %s", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("cannot create Kubernetes client config: %s", err)
	}

	node, err := client.CoreV1().Nodes().Get(context.Background(), n.nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to find node by name %q: %w", n.nodeName, err)
	}

	zone, ok := node.Labels[wellKnownTopologyKeyZone]
	if !ok {
		zone, ok = node.Labels[failureDomainZoneLabel]
		if !ok {
			return "", fmt.Errorf("zone label not found in metadata of node %s", n.nodeName)
		}
	}

	return zone, nil
}

func (n *nodeService) isBlockDevice(fullPath string) (bool, error) {
	var st unix.Stat_t
	err := unix.Stat(fullPath, &st)
	if err != nil {
		return false, err
	}

	return (st.Mode & unix.S_IFMT) == unix.S_IFBLK, nil
}

func (n *nodeService) getBlockSizeBytes(devicePath string) (int64, error) {
	cmd := n.mounter.(*NodeMounter).Exec.Command("blockdev", "--getsize64", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return -1, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %v", devicePath, string(output), err)
	}

	strOut := strings.TrimSpace(string(output))
	gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse size %s as int", strOut)
	}

	return gotSizeBytes, nil
}

func getFsInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	statfs := &unix.Statfs_t{}
	err := unix.Statfs(path, statfs)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	// Available is blocks available * fragment size
	available := int64(statfs.Bavail) * statfs.Bsize

	// Capacity is total block count * fragment size
	capacity := int64(statfs.Blocks) * statfs.Bsize

	// Usage is block being used * fragment size (aka block size).
	usage := (int64(statfs.Blocks) - int64(statfs.Bfree)) * statfs.Bsize

	inodes := int64(statfs.Files)
	inodesFree := int64(statfs.Ffree)
	inodesUsed := inodes - inodesFree

	return available, capacity, usage, inodes, inodesFree, inodesUsed, nil
}

func isNotMountedError(err error) bool {
	return strings.Contains(err.Error(), "not mounted")
}
