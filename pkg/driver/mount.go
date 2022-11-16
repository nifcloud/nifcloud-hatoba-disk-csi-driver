package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	mountutils "k8s.io/mount-utils"
	"k8s.io/utils/exec"
)

// Mounter is an interface for mount operations
type Mounter interface {
	mountutils.Interface

	FormatAndMount(source string, target string, fstype string, options []string) error
	GetDeviceName(mountPath string) (string, int, error)
	MakeFile(pathname string) error
	MakeDir(pathname string) error
	ExistsPath(filename string) (bool, error)
	IsCorruptedMnt(err error) bool
	NeedResize(devicePath string, deviceMountPath string) (bool, error)
	NewResizeFs() (Resizefs, error)
	FindDevicePath(scsiID string) (string, error)
	ScanStorageDevices() error
	RemoveStorageDevice(dev string) error
	RescanStorageDevice(dev string) error
}

type Resizefs interface {
	Resize(devicePath, deviceMountPath string) (bool, error)
}

// NodeMounter mount the devices.
type NodeMounter struct {
	mountutils.SafeFormatAndMount
}

func newNodeMounter() Mounter {
	return &NodeMounter{
		mountutils.SafeFormatAndMount{
			Interface: mountutils.New(""),
			Exec:      exec.New(),
		},
	}
}

// GetDeviceName returns the device name from path
func (m *NodeMounter) GetDeviceName(mountPath string) (string, int, error) {
	return mountutils.GetDeviceNameFromMount(m, mountPath)
}

func (m *NodeMounter) MakeFile(pathname string) error {
	f, err := os.OpenFile(pathname, os.O_CREATE, os.FileMode(0644))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err = f.Close(); err != nil {
		return err
	}
	return nil
}

func (m *NodeMounter) MakeDir(pathname string) error {
	err := os.MkdirAll(pathname, os.FileMode(0755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func (m *NodeMounter) ExistsPath(filename string) (bool, error) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (m *NodeMounter) IsCorruptedMnt(err error) bool {
	return mountutils.IsCorruptedMnt(err)
}

func (m *NodeMounter) NeedResize(devicePath string, deviceMountPath string) (bool, error) {
	return mountutils.NewResizeFs(m.Exec).NeedResize(devicePath, deviceMountPath)
}

func (m *NodeMounter) NewResizeFs() (Resizefs, error) {
	return mountutils.NewResizeFs(m.Exec), nil
}

func (m *NodeMounter) FindDevicePath(scsiID string) (string, error) {
	if !strings.HasPrefix(scsiID, "SCSI") {
		return "", fmt.Errorf("invalid SCSI ID %q was specified. SCSI ID must be start with SCSI (0:?)", scsiID)
	}
	deviceNumberRegexp := regexp.MustCompile(`SCSI\s\(0:(.+)\)$`)
	match := deviceNumberRegexp.FindSubmatch([]byte(scsiID))
	if match == nil {
		return "", fmt.Errorf("could not detect device file from SCSI id %q", scsiID)
	}
	deviceNumber := string(match[1])

	deviceFileDir := "/dev/disk/by-path"
	files, err := os.ReadDir(deviceFileDir)
	if err != nil {
		return "", fmt.Errorf("could not list the files in /dev/disk/by-path/: %v", err)
	}

	devicePath := ""
	deviceFileRegexp := regexp.MustCompile(fmt.Sprintf(`^pci-\d{4}:\d{2}:\d{2}\.\d-scsi-0:0:%s:0$`, deviceNumber))
	for _, f := range files {
		if deviceFileRegexp.MatchString(f.Name()) {
			devicePath, err = filepath.EvalSymlinks(filepath.Join(deviceFileDir, f.Name()))
			if err != nil {
				return "", fmt.Errorf("could not eval symlynk for %q: %v", f.Name(), err)
			}
		}
	}

	if devicePath == "" {
		return "", fmt.Errorf("could not find device file from SCSI ID %q", scsiID)
	}

	exists, err := m.ExistsPath(devicePath)
	if err != nil {
		return "", err
	}

	if exists {
		return devicePath, nil
	}

	return "", fmt.Errorf("device path not found: %s", devicePath)
}

// ScanStorageDevices online scan the new storage device
// More info: https://pfs.nifcloud.com/guide/cp/login/mount_linux.htm
func (m *NodeMounter) ScanStorageDevices() error {
	scanTargets := []string{"/sys/class/scsi_host", "/sys/devices"}
	for _, target := range scanTargets {
		err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if info.Name() != "scan" {
				return nil
			}

			f, err := os.OpenFile(path, os.O_WRONLY, os.ModePerm)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = fmt.Fprint(f, "- - -")
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan devices: %v", err)
		}
	}

	return nil
}

// RemoveStorageDevice online detach the specified storage
// More info: https://pfs.nifcloud.com/guide/cp/login/detach_linux.htm
func (m *NodeMounter) RemoveStorageDevice(dev string) error {
	removeDevicePath := filepath.Join("/sys/block/", filepath.Base(dev), "/device/delete")
	if _, err := os.Stat(removeDevicePath); err != nil {
		// If the path does not exist, assume it is removed from this node
		return nil
	}

	f, err := os.OpenFile(removeDevicePath, os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprint(f, "1")
	if err != nil {
		return err
	}

	return nil
}

// RescanStorageDevice online rescan the specified storage
// More info: https://pfs.nifcloud.com/guide/cp/login/extend_partition_linux.htm
func (m *NodeMounter) RescanStorageDevice(dev string) error {
	rescanDevicePath := filepath.Join("/sys/block/", filepath.Base(dev), "/device/rescan")
	if _, err := os.Stat(rescanDevicePath); err != nil {
		return fmt.Errorf("Target device %q not found in /sys/block: %w", dev, err)
	}

	f, err := os.OpenFile(rescanDevicePath, os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprint(f, "1"); err != nil {
		return err
	}

	return nil
}
