package cloud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/logging"
	"github.com/google/uuid"
	"github.com/nifcloud/nifcloud-hatoba-disk-csi-driver/pkg/util"
	"github.com/nifcloud/nifcloud-sdk-go/nifcloud"
	"github.com/nifcloud/nifcloud-sdk-go/service/hatoba"
	"github.com/nifcloud/nifcloud-sdk-go/service/hatoba/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

const (
	// Available disk types
	// VolumeTypeStandardFlash represents a standard flash volume (randomly select type A or B)
	VolumeTypeStandardFlash = "standard-flash"
	// VolumeTypeStandardFlashA represents a standard flash volume (only use type A)
	VolumeTypeStandardFlashA = "standard-flash-a"
	// VolumeTypeStandardFlashB represents a standard flash volume (only use type B)
	VolumeTypeStandardFlashB = "standard-flash-b"
	// VolumeTypeHighSpeedFlash represents a high spped flash volume (randomly select type A or B)
	VolumeTypeHighSpeedFlash = "high-speed-flash"
	// VolumeTypeHighSpeedFlashA represents a high spped flash volume (only use type A)
	VolumeTypeHighSpeedFlashA = "high-speed-flash-a"
	// VolumeTypeHighSpeedFlashB represents a high spped flash volume (only use type B)
	VolumeTypeHighSpeedFlashB = "high-speed-flash-b"
)

const (
	// DefaultVolumeSize represents the default volume size.
	DefaultVolumeSize int64 = 100 * util.GiB
	// DefaultVolumeType specifies which storage to use for newly created volumes.
	DefaultVolumeType = VolumeTypeStandardFlashA

	// TagKeyCSIVolumeName is the tag key for CSI volume name.
	TagKeyCSIVolumeName = "disk.csi.hatoba.nifcloud.com/csi-volume-name"

	diskReadyState    = "READY"
	diskAttachedState = "ATTACHED"
	diskDetachedState = "DETACHED"
)

var (
	// ErrMultiDisks is an error that is returned when multiple
	// disks are found with the same volume name.
	ErrMultiDisks = errors.New("Multiple disks with same name")

	// ErrDiskExistsDiffSize is an error that is returned if a disk with a given
	// name, but different size, is found.
	ErrDiskExistsDiffSize = errors.New("There is already a disk with same name and different size")

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = errors.New("Resource was not found")

	// ErrAlreadyExists is returned when a resource is already existent.
	ErrAlreadyExists = errors.New("Resource already exists")

	// ErrVolumeInUse is returned when a disk is already attached to an node.
	ErrDiskInUse = errors.New("Request disk is already attached to an node")

	checkInterval = 5 * time.Second
	// This timeout can be "ovewritten" if the value returned by ctx.Deadline()
	// comes sooner. That value comes from the external provisioner controller.
	checkTimeout = 10 * time.Minute
)

// Disk represents a NIFCLOUD Hatoba disk
type Volume struct {
	VolumeID         string
	CapacityGiB      int64
	AvailabilityZone string
}

// DiskOptions represents parameters to create an NIFCLOUD Hatoba disk
type DiskOptions struct {
	CapacityBytes int64
	VolumeType    string
	Zone          string
	Tags          map[string]string
}

// Cluster represents a NIFCLOUD Hatoba cluster
type Cluster struct {
	Name      string
	NodePools []*NodePool
}

// NodePool represents a pool of nodes in cluster
type NodePool struct {
	Name  string
	Nodes []*Node
}

// Node represents a node in cluster
type Node struct {
	Name            string
	PublicIPAddress string
}

// Cloud is interface for cloud api manipulator
type Cloud interface {
	CreateDisk(ctx context.Context, clusterName string, diskOptions *DiskOptions) (disk *Volume, err error)
	DeleteDisk(ctx context.Context, volumeID string) (success bool, err error)
	AttachDisk(ctx context.Context, volumeID, nodeID string) (devicePath string, err error)
	DetachDisk(ctx context.Context, volumeID, nodeID string) (err error)
	ResizeDisk(ctx context.Context, volumeID string, size int64) (int64, error)
	GetDiskByName(ctx context.Context, name string, capacityBytes int64) (disk *Volume, err error)
	GetDiskByID(ctx context.Context, volumeID string) (disk *Volume, err error)
	IsExistNode(ctx context.Context, clusterName, nodeID string) (success bool)
	GetClusterName(ctx context.Context, nrn string) (string, error)
}

type cloud struct {
	client *client
	sdk    *hatoba.Client
}

var _ Cloud = &cloud{}

type logger struct{}

var _ logging.Logger = &logger{}

func (l logger) Logf(classification logging.Classification, format string, v ...interface{}) {
	if len(classification) != 0 {
		format = string(classification) + " " + format
	}
	klog.V(4).Infof(format, v...)
}

// NewCloud creates the cloud object.
func NewCloud(region string) (Cloud, error) {
	accessKeyID := os.Getenv("NIFCLOUD_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("NIFCLOUD_SECRET_ACCESS_KEY")
	if accessKeyID == "" {
		return nil, fmt.Errorf("NIFCLOUD_ACCESS_KEY_ID is required")
	}
	if secretAccessKey == "" {
		return nil, fmt.Errorf("NIFCLOUD_SECRET_ACCESS_KEY is required")
	}
	if region == "" {
		return nil, fmt.Errorf("NIFCLOUD_REGION is required")
	}

	cfg := nifcloud.NewConfig(accessKeyID, secretAccessKey, region)
	cfg.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
	cfg.Logger = &logger{}

	endpoint := os.Getenv("NIFCLOUD_HATOBA_ENDPOINT")
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.hatoba.api.nifcloud.com", region)
	} else {
		cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           endpoint,
					SigningRegion: region,
				}, nil
			},
		)
	}

	return &cloud{
		client: newClient(region, accessKeyID, secretAccessKey, endpoint),
		sdk:    hatoba.NewFromConfig(cfg),
	}, nil
}

func (c *cloud) CreateDisk(ctx context.Context, clusterName string, diskOptions *DiskOptions) (*Volume, error) {
	zone := diskOptions.Zone
	if zone == "" {
		return nil, errors.New("Zone is required")
	}
	capacity := roundUpCapacity(util.BytesToGiB(diskOptions.CapacityBytes))

	tags := []*Tag{}
	for key, value := range diskOptions.Tags {
		tags = append(tags, &Tag{
			Key:   key,
			Value: value,
		})
	}

	resp, err := c.client.CreateDisk(&CreateDiskRequest{
		Disk: &DiskToCreate{
			Name: strings.Replace(uuid.New().String(), "-", "", -1),
			Cluster: &ClusterForDisk{
				Name: clusterName,
			},
			Size:             uint(capacity),
			Type:             diskOptions.VolumeType,
			AvailabilityZone: zone,
			Tags:             tags,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not create NIFCLOUD Hatoba disk: %v", err)
	}

	if resp.Disk == nil {
		return nil, fmt.Errorf("CreateDisk response is nil")
	}

	name := resp.Disk.Name
	if len(name) == 0 {
		return nil, fmt.Errorf("disk name was not returned by CreateDisk")
	}

	createdZone := resp.Disk.AvailabilityZone
	if len(zone) == 0 {
		return nil, fmt.Errorf("availability zone was not returned by CreateDisk")
	}

	createdSize := resp.Disk.Size
	if createdSize == 0 {
		return nil, fmt.Errorf("disk size was not returned by CreateDisk")
	}

	if err := c.waitForDisk(ctx, name); err != nil {
		return nil, fmt.Errorf("failed to get an READY disk: %v", err)
	}

	return &Volume{
		CapacityGiB:      int64(createdSize),
		VolumeID:         name,
		AvailabilityZone: createdZone,
	}, nil
}

func (c *cloud) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	if err := c.client.DeleteDisk(&DeleteDiskRequest{DiskName: volumeID}); err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("DeleteDisk could not delete disk: %w", err)
	}
	return true, nil
}

func (c *cloud) AttachDisk(ctx context.Context, volumeID, nodeID string) (string, error) {
	if err := c.client.AttachDisk(
		&AttachDiskRequest{
			DiskName: volumeID,
			NodeName: nodeID,
		},
	); err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return "", ErrNotFound
		}
		if strings.Contains(err.Error(), "Client.ResourceIncorrectState.Disk.Attached") {
			return "", ErrDiskInUse
		}
		return "", fmt.Errorf("could not attach disk %q to node %q: %v", volumeID, nodeID, err)
	}
	klog.V(5).Infof("AttachDisk disk=%q node=%q", volumeID, nodeID)

	// This is the only situation where we taint the device
	if err := c.waitForDiskAttachment(ctx, volumeID, diskAttachedState); err != nil {
		return "", err
	}

	resp, err := c.client.GetDisk(&GetDiskRequest{DiskName: volumeID})
	if err != nil {
		return "", fmt.Errorf("failed to get disk: %w", err)
	}

	if resp.Disk == nil || len(resp.Disk.Attachments) == 0 || resp.Disk.Attachments[0].DevicePath == "" {
		return "", fmt.Errorf("disk response does not include the device path")
	}

	return resp.Disk.Attachments[0].DevicePath, nil
}

func (c *cloud) DetachDisk(ctx context.Context, volumeID, nodeID string) error {
	if err := c.client.DetachDisk(&DetachDiskRequest{DiskName: volumeID, NodeName: nodeID}); err != nil {
		if strings.Contains(err.Error(), "NotFound") ||
			strings.Contains(err.Error(), "Client.ResourceIncorrectState.Disk.Detached") {
			return ErrNotFound
		}
		return fmt.Errorf("could not detach disk %q from node %q: %w", volumeID, nodeID, err)
	}
	klog.V(5).Infof("DetachDisk disk=%q node=%q", volumeID, nodeID)

	// This is the only situation where we taint the device
	if err := c.waitForDiskAttachment(ctx, volumeID, diskDetachedState); err != nil {
		return err
	}

	return nil
}

func (c *cloud) ResizeDisk(ctx context.Context, volumeID string, size int64) (int64, error) {
	disk, err := c.GetDiskByID(ctx, volumeID)
	if err != nil {
		return 0, err
	}

	currentSize := disk.CapacityGiB
	desiredSize := roundUpCapacity(util.BytesToGiB(size))
	if desiredSize-currentSize < 0 {
		return 0, fmt.Errorf(
			"could not resize %s's size to %d because it is smaller than current size %d",
			volumeID, desiredSize, currentSize,
		)
	} else if currentSize >= desiredSize {
		// no need to resize.
		return currentSize, nil
	}

	if _, err := c.client.UpdateDisk(&UpdateDiskRequest{
		Name: volumeID,
		Disk: &DiskToUpdate{
			Size: uint(desiredSize),
		},
	}); err != nil {
		return 0, err
	}

	if err := c.waitForDisk(ctx, volumeID); err != nil {
		return 0, fmt.Errorf("failed to get an READY disk: %v", err)
	}

	// fetch latest volume status.
	disk, err = c.GetDiskByID(ctx, volumeID)
	if err != nil {
		return 0, err
	}

	klog.V(4).Infof("resize succeeded! current disk size is %dGiB", disk.CapacityGiB)

	return disk.CapacityGiB, nil
}

func (c *cloud) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*Volume, error) {
	resp, err := c.client.ListDisks(fmt.Sprintf("tags.%s=%s", TagKeyCSIVolumeName, name))
	if err != nil {
		return nil, fmt.Errorf("could not list the disks: %v", err)
	}
	if resp == nil || len(resp.Disks) == 0 {
		return nil, ErrNotFound
	}

	disk := resp.Disks[0]
	if int64(disk.Size) != roundUpCapacity(util.BytesToGiB(capacityBytes)) {
		klog.Warningf(
			"disk size for %q is not same. request capacityBytes: %v != disk size: %v",
			name, roundUpCapacity(util.BytesToGiB(capacityBytes)), disk.Size,
		)
		return nil, ErrDiskExistsDiffSize
	}

	return &Volume{
		VolumeID:         disk.Name,
		CapacityGiB:      int64(disk.Size),
		AvailabilityZone: disk.AvailabilityZone,
	}, nil
}

func (c *cloud) GetDiskByID(ctx context.Context, volumeID string) (*Volume, error) {
	input := &GetDiskRequest{
		DiskName: volumeID,
	}

	disk, err := c.getDisk(ctx, input)
	if err != nil {
		return nil, err
	}

	return &Volume{
		VolumeID:         disk.Name,
		CapacityGiB:      int64(disk.Size),
		AvailabilityZone: disk.AvailabilityZone,
	}, nil
}

func (c *cloud) IsExistNode(ctx context.Context, clusterName, nodeID string) bool {
	node, err := c.getNode(ctx, clusterName, nodeID)
	if err != nil || node == nil {
		return false
	}
	return true
}

func (c *cloud) GetClusterName(ctx context.Context, nrn string) (string, error) {
	// custom options to avoid the behavior of the SDK double-escaping colons(:).
	opts := []func(*hatoba.Options){
		func(opt *hatoba.Options) {
			opt.HTTPSignerV4 = v4.NewSigner(func(so *v4.SignerOptions) {
				so.Logger = opt.Logger
				so.LogSigning = opt.ClientLogMode.IsSigning()
				so.DisableURIPathEscaping = true
			})
		},
	}

	res, err := c.sdk.GetCluster(
		ctx,
		&hatoba.GetClusterInput{
			ClusterName: nifcloud.String(nrn),
		},
		opts...,
	)
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) && apiError.ErrorCode() == "Client.InvalidParameterNotFound.Cluster" {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("GetCluster returns an error: %w", err)
	}

	return nifcloud.ToString(res.Cluster.Name), nil
}

// waitForDisk waits for disk status to be available.
func (c *cloud) waitForDisk(ctx context.Context, volumeID string) error {
	input := &GetDiskRequest{
		DiskName: volumeID,
	}

	err := wait.Poll(checkInterval, checkTimeout, func() (done bool, err error) {
		disk, err := c.getDisk(ctx, input)
		if err != nil {
			return true, err
		}
		return disk.Status == diskReadyState, nil
	})

	return err
}

// waitForDiskAttachment polls until the attachment status is the expected value.
func (c *cloud) waitForDiskAttachment(ctx context.Context, volumeID, expectedState string) error {
	input := &GetDiskRequest{
		DiskName: volumeID,
	}

	err := wait.Poll(checkInterval, checkTimeout, func() (done bool, err error) {
		disk, err := c.getDisk(ctx, input)
		if err != nil {
			return true, err
		}

		var currentState string
		if len(disk.Attachments) == 0 {
			currentState = diskDetachedState
		} else {
			currentState = disk.Attachments[0].Status
		}

		return currentState == expectedState, nil
	})

	return err
}

func (c *cloud) getNode(ctx context.Context, clusterName string, nodeID string) (*types.Nodes, error) {
	response, err := c.sdk.GetCluster(ctx, &hatoba.GetClusterInput{
		ClusterName: nifcloud.String(clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting NIFCLOUD cluster: %v", err)
	}

	for _, np := range response.Cluster.NodePools {
		for _, node := range np.Nodes {
			if nifcloud.ToString(node.Name) == nodeID {
				return &node, nil
			}
		}
	}

	return nil, fmt.Errorf("not found node %q in cluster %q", nodeID, clusterName)
}

func (c *cloud) getDisk(ctx context.Context, input *GetDiskRequest) (*Disk, error) {
	response, err := c.client.GetDisk(input)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if response.Disk == nil {
		return nil, ErrNotFound
	}

	return response.Disk, nil
}

func roundUpCapacity(capacityGiB int64) int64 {
	// NIFCLOUD Hatoba disk unit
	// 100, 200, 300, ... 1000
	const unit = 100

	if capacityGiB%unit == 0 {
		return capacityGiB
	}
	return (capacityGiB/unit + 1) * unit
}
