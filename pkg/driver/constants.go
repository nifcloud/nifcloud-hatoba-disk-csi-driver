package driver

// constants for default command line flag values
const (
	DefaultCSIEndpoint = "unix://tmp/csi.sock"

	// Parameters for StorageClass.
	VolumeTypeKey = "type"

	// Keys for PV and PVC parameters as reported by external-provisioner.
	parameterKeyPVCName      = "csi.storage.k8s.io/pvc/name"
	parameterKeyPVCNamespace = "csi.storage.k8s.io/pvc/namespace"
	parameterKeyPVName       = "csi.storage.k8s.io/pv/name"

	// Keys for tags to put in the provisioned disk description.
	tagKeyCreatedForClaimNamespace    = "kubernetes.io/created-for/pvc/namespace"
	tagKeyCreatedForClaimName         = "kubernetes.io/created-for/pvc/name"
	tagKeyCreatedForVolumeName        = "kubernetes.io/created-for/pv/name"
	tagKeyCreatedBy                   = "disk.csi.hatoba.nifcloud.com/created-by"
	tagKeyCSIVolumeName               = "disk.csi.hatoba.nifcloud.com/csi-volume-name"
	tagKeyNIFCLOUDHatobaDiskCSIDriver = "disk.hatoba.csi.nifcloud.com/cluster"

	devicePathKey = "devicePath"

	// well-known label keys.
	wellKnownTopologyKeyZone = "topology.kubernetes.io/zone"
)
