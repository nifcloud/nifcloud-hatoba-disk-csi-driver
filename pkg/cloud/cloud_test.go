package cloud

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nifcloud/nifcloud-hatoba-disk-csi-driver/pkg/util"
	"github.com/nifcloud/nifcloud-sdk-go/nifcloud"
	"github.com/nifcloud/nifcloud-sdk-go/service/hatoba"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	dummyAccessKeyID     = "accesskeyfortesting"
	dummySecretAccessKey = "secretaccesskeyfortesting"

	defaultRegion   = "jp-east-1"
	defaultZone     = "east-11"
	defaultDiskSize = 100

	testClusterName = "test-cluster"
	testNodeName    = "test-node-01"
)

func modifyCheckIntervalAndTimeoutForTesting() func() {
	orgCheckInterval := checkInterval
	orgCheckTimeout := checkTimeout

	checkInterval = 1 * time.Second
	checkTimeout = 5 * time.Second

	return func() {
		checkInterval = orgCheckInterval
		checkTimeout = orgCheckTimeout
	}
}

func diskListAPIResponse(name string, size int, status string) string {
	return fmt.Sprintf(
		`{"disks": [{"name": "%s", "availabilityZone": "%s", "size": %d, "status": "%s"}]}`,
		name, defaultZone, size, status,
	)
}

func diskAPIResponse(name string, size int, status string) string {
	return fmt.Sprintf(
		`{"disk": {"name": "%s", "availabilityZone": "%s", "size": %d, "status": "%s"}}`,
		name, defaultZone, size, status,
	)
}

func diskAPIResponseWithAttachments(name string, size int, nodeName, devicePath, attachmentStatus string) string {
	return fmt.Sprintf(
		`{"disk": {"name": "%s", "availabilityZone": "%s", "size": %d, "status": "READY", "attachments": [{"nodeName": "%s", "devicePath": "%s", "status": "%s"}]}}`,
		name, defaultZone, size, nodeName, devicePath, attachmentStatus,
	)
}

func errorResponse(code, message string) string {
	return fmt.Sprintf(`{"code": "%s", "message": "%s"}`, code, message)
}

func maintenanceResponse() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"code": "Server.Maintenance", "message": "The system is in maintenance now."}`)
		w.Header().Add("content-type", "application/json")
	})
}

func createSDKClientForTesting(url string) *hatoba.Client {
	cfg := nifcloud.NewConfig(dummyAccessKeyID, dummySecretAccessKey, defaultRegion)
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           url,
				SigningRegion: region,
			}, nil
		},
	)

	return hatoba.NewFromConfig(cfg)
}

func createClientForTesting(url string) *client {
	return newClient(defaultRegion, dummyAccessKeyID, dummySecretAccessKey, url)
}

func TestNewCloud(t *testing.T) {
	tests := []struct {
		name         string
		region       string
		env          map[string]string
		wantEndpoint string
		wantErr      bool
	}{
		{
			name:   "success: create cloud",
			region: defaultRegion,
			env: map[string]string{
				"NIFCLOUD_ACCESS_KEY_ID":     dummyAccessKeyID,
				"NIFCLOUD_SECRET_ACCESS_KEY": dummySecretAccessKey,
			},
			wantEndpoint: "https://jp-east-1.hatoba.api.nifcloud.com",
		},
		{
			name:   "success: create cloud with endpoint",
			region: defaultRegion,
			env: map[string]string{
				"NIFCLOUD_ACCESS_KEY_ID":     dummyAccessKeyID,
				"NIFCLOUD_SECRET_ACCESS_KEY": dummySecretAccessKey,
				"NIFCLOUD_HATOBA_ENDPOINT":   "http://192.168.0.1",
			},
			wantEndpoint: "http://192.168.0.1",
		},
		{
			name:   "error: region is empty",
			region: "",
			env: map[string]string{
				"NIFCLOUD_ACCESS_KEY_ID":     dummyAccessKeyID,
				"NIFCLOUD_SECRET_ACCESS_KEY": dummySecretAccessKey,
			},
			wantErr: true,
		},
		{
			name:   "error: NIFCLOUD_ACCESS_KEY_ID is empty",
			region: defaultRegion,
			env: map[string]string{
				"NIFCLOUD_SECRET_ACCESS_KEY": dummySecretAccessKey,
			},
			wantErr: true,
		},
		{
			name:   "error: NIFCLOUD_SECRET_ACCESS_KEY is empty",
			region: defaultRegion,
			env: map[string]string{
				"NIFCLOUD_ACCESS_KEY_ID": dummyAccessKeyID,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, val := range tt.env {
				os.Setenv(key, val)
			}
			defer func() {
				for key := range tt.env {
					os.Unsetenv(key)
				}
			}()

			got, err := NewCloud(tt.region)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			c, ok := got.(*cloud)
			require.True(t, ok)
			assert.NotNil(t, c.sdk)
			assert.NotNil(t, c.client)

			sdkElm := reflect.ValueOf(c.sdk).Elem()
			optionsField := sdkElm.FieldByName("options")
			optionsField = reflect.NewAt(optionsField.Type(), unsafe.Pointer(optionsField.UnsafeAddr())).Elem()
			options := optionsField.Interface().(hatoba.Options)
			ep, err := options.EndpointResolver.ResolveEndpoint(tt.region, hatoba.EndpointResolverOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantEndpoint, ep.URL)
		})
	}
}

func TestCreateDisk(t *testing.T) {
	reset := modifyCheckIntervalAndTimeoutForTesting()
	defer reset()

	diskName := uuid.New().String()
	requestedDiskSizeInBytes := int64(10 * util.GiB)

	diskOptions := &DiskOptions{
		CapacityBytes: requestedDiskSizeInBytes,
		VolumeType:    VolumeTypeStandardFlash,
		Zone:          defaultZone,
		Tags: map[string]string{
			"csi.storage.k8s.io/pv/name": "test-pv",
		},
	}

	tests := []struct {
		name              string
		diskOptions       *DiskOptions
		mockServerHandler func(t *testing.T) http.Handler
		want              *Volume
		wantErr           error
	}{
		{
			name:        "success: create new disk",
			diskOptions: diskOptions,
			mockServerHandler: func(t *testing.T) http.Handler {
				getDiskCallCount := 0
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)
					fmt.Fprint(w, diskAPIResponse(diskName, defaultDiskSize, "CREATING"))
				}).Methods(http.MethodPost)

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					getDiskCallCount++
					// wait until disk to be ready.
					if getDiskCallCount < 2 {
						fmt.Fprint(w, diskAPIResponse(diskName, defaultDiskSize, "CREATING"))
						return
					}
					fmt.Fprint(w, diskAPIResponse(diskName, defaultDiskSize, "READY"))
				}).Methods(http.MethodGet)

				return r
			},
			want: &Volume{
				VolumeID:         diskName,
				CapacityGiB:      int64(defaultDiskSize),
				AvailabilityZone: defaultZone,
			},
		},
		{
			name:        "error: server is in maintenance",
			diskOptions: diskOptions,
			mockServerHandler: func(t *testing.T) http.Handler {
				return maintenanceResponse()
			},
			wantErr: fmt.Errorf("could not create NIFCLOUD Hatoba disk: failed create disk: [503 Service Unavailable] Server.Maintenance: The system is in maintenance now."), // nolint: revive
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				client: createClientForTesting(ts.URL),
			}
			got, err := c.CreateDisk(context.Background(), testClusterName, tt.diskOptions)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDeleteDisk(t *testing.T) {
	diskName := uuid.New().String()

	tests := []struct {
		name              string
		mockServerHandler func(t *testing.T) http.Handler
		want              bool
		wantErr           error
	}{
		{
			name: "success: delete disk",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, diskAPIResponse(diskName, defaultDiskSize, "DELETING"))
				}).Methods(http.MethodDelete)

				return r
			},
			want: true,
		},
		{
			name: "error: disk was already deleted",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, errorResponse(
						"Client.InvalidParameterNotFound.Disk",
						fmt.Sprintf("The disk '%s' could not be found.", diskName)),
					)
				}).Methods(http.MethodDelete)

				return r
			},
			want:    false,
			wantErr: ErrNotFound,
		},
		{
			name: "error: server is in maintenance",
			mockServerHandler: func(t *testing.T) http.Handler {
				return maintenanceResponse()
			},
			want:    false,
			wantErr: fmt.Errorf("DeleteDisk could not delete disk: failed delete disk: [503 Service Unavailable] Server.Maintenance: The system is in maintenance now."), // nolint: revive
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				client: createClientForTesting(ts.URL),
			}
			got, err := c.DeleteDisk(context.Background(), diskName)

			assert.Equal(t, tt.want, got)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAttachDisk(t *testing.T) {
	reset := modifyCheckIntervalAndTimeoutForTesting()
	defer reset()

	diskName := uuid.New().String()
	devicePath := "SCSI (0:1)"

	tests := []struct {
		name              string
		mockServerHandler func(t *testing.T) http.Handler
		want              string
		wantErr           error
	}{
		{
			name: "success: attach disk",
			mockServerHandler: func(t *testing.T) http.Handler {
				getDiskCallCount := 0
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}:attach", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, diskAPIResponseWithAttachments(diskName, defaultDiskSize, testNodeName, "", "ATTACHING"))
				}).Methods(http.MethodPost)

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					getDiskCallCount++
					// wait until disk to be attached.
					if getDiskCallCount < 2 {
						fmt.Fprint(w, diskAPIResponseWithAttachments(diskName, defaultDiskSize, testNodeName, "", "ATTACHING"))
						return
					}
					fmt.Fprint(w, diskAPIResponseWithAttachments(diskName, defaultDiskSize, testNodeName, devicePath, "ATTACHED"))
				}).Methods(http.MethodGet)

				return r
			},
			want: devicePath,
		},
		{
			name: "error: disk is already attached to node",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}:attach", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, errorResponse(
						"Client.ResourceIncorrectState.Disk.Attached",
						fmt.Sprintf("Cannot operate the disk '%s' because the disk is already attached to node.", diskName),
					))
				}).Methods(http.MethodPost)

				return r
			},
			wantErr: ErrDiskInUse,
		},
		{
			name: "error: server is in maintenance",
			mockServerHandler: func(t *testing.T) http.Handler {
				return maintenanceResponse()
			},
			wantErr: fmt.Errorf("could not attach disk \"%s\" to node \"%s\": failed attach disk: [503 Service Unavailable] Server.Maintenance: The system is in maintenance now.", diskName, testNodeName), // nolint: revive
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				client: createClientForTesting(ts.URL),
			}
			got, err := c.AttachDisk(context.Background(), diskName, testNodeName)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDetachDisk(t *testing.T) {
	reset := modifyCheckIntervalAndTimeoutForTesting()
	defer reset()

	diskName := uuid.New().String()

	tests := []struct {
		name              string
		mockServerHandler func(t *testing.T) http.Handler
		wantErr           error
	}{
		{
			name: "success: detach disk",
			mockServerHandler: func(t *testing.T) http.Handler {
				getDiskCallCount := 0
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}:detach", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, diskAPIResponseWithAttachments(diskName, defaultDiskSize, testNodeName, "", "DETACHING"))
				}).Methods(http.MethodPost)

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					getDiskCallCount++
					// wait until disk to be detached.
					if getDiskCallCount < 2 {
						fmt.Fprint(w, diskAPIResponseWithAttachments(diskName, defaultDiskSize, testNodeName, "", "DETACHING"))
						return
					}
					fmt.Fprint(w, diskAPIResponse(diskName, defaultDiskSize, "READY"))
				}).Methods(http.MethodGet)

				return r
			},
		},
		{
			name: "error: disk is already detached from node",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}:detach", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, errorResponse(
						"Client.ResourceIncorrectState.Disk.Detached",
						fmt.Sprintf("Cannot operate the disk '%s' because the disk is already detached from node.", diskName),
					))
				}).Methods(http.MethodPost)

				return r
			},
			wantErr: ErrNotFound,
		},
		{
			name: "error: server is in maintenance",
			mockServerHandler: func(t *testing.T) http.Handler {
				return maintenanceResponse()
			},
			wantErr: fmt.Errorf("could not detach disk \"%s\" from node \"%s\": failed detach disk: [503 Service Unavailable] Server.Maintenance: The system is in maintenance now.", diskName, testNodeName), // nolint: revive
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				client: createClientForTesting(ts.URL),
			}
			err := c.DetachDisk(context.Background(), diskName, testNodeName)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestResizeDisk(t *testing.T) {
	reset := modifyCheckIntervalAndTimeoutForTesting()
	defer reset()

	diskName := uuid.New().String()

	tests := []struct {
		name              string
		mockServerHandler func(t *testing.T) http.Handler
		diskSize          int64
		want              int64
		wantErr           error
	}{
		{
			name: "success: resize disk",
			mockServerHandler: func(t *testing.T) http.Handler {
				getDiskCallCount := 0
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, diskAPIResponse(diskName, defaultDiskSize, "RESIZING"))
				}).Methods(http.MethodPut)

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					getDiskCallCount++
					if getDiskCallCount == 1 {
						// to get the current disk info
						fmt.Fprint(w, diskAPIResponse(diskName, defaultDiskSize, "READY"))
						return
					} else if getDiskCallCount < 3 {
						// waiting for resized
						fmt.Fprint(w, diskAPIResponse(diskName, 200, "RESIZING"))
						return
					}
					// resized
					fmt.Fprint(w, diskAPIResponse(diskName, 200, "READY"))
				}).Methods(http.MethodGet)

				return r
			},
			diskSize: util.GiBToBytes(110),
			want:     200,
		},
		{
			name: "error: requested size is smaller than current size",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, diskAPIResponse(diskName, 200, "READY"))
				}).Methods(http.MethodGet)

				return r
			},
			diskSize: util.GiBToBytes(100),
			wantErr:  fmt.Errorf("could not resize %s's size to 100 because it is smaller than current size 200", diskName),
		},
		{
			name: "error: server is in maintenance",
			mockServerHandler: func(t *testing.T) http.Handler {
				return maintenanceResponse()
			},
			wantErr: fmt.Errorf("failed get disk: [503 Service Unavailable] Server.Maintenance: The system is in maintenance now."), // nolint: revive
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				client: createClientForTesting(ts.URL),
			}
			got, err := c.ResizeDisk(context.Background(), diskName, tt.diskSize)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetDiskByName(t *testing.T) {
	pvName := "test-pv-name"

	assertTagFilterEqual := func(t *testing.T, query url.Values) {
		requestedFilter := query.Get("filters")
		assert.Equal(t, fmt.Sprintf("tags.%s=%s", TagKeyCSIVolumeName, pvName), requestedFilter)
	}

	tests := []struct {
		name              string
		mockServerHandler func(t *testing.T) http.Handler
		capacityBytes     int64
		want              *Volume
		wantErr           error
	}{
		{
			name: "success: get disk by name",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks", func(w http.ResponseWriter, r *http.Request) {
					assertTagFilterEqual(t, r.URL.Query())
					fmt.Fprint(w, diskListAPIResponse(pvName, defaultDiskSize, "READY"))
				}).Methods(http.MethodGet)

				return r
			},
			capacityBytes: util.GiBToBytes(defaultDiskSize),
			want: &Volume{
				VolumeID:         pvName,
				CapacityGiB:      defaultDiskSize,
				AvailabilityZone: defaultZone,
			},
		},
		{
			name: "error: disk is not found",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks", func(w http.ResponseWriter, r *http.Request) {
					assertTagFilterEqual(t, r.URL.Query())
					fmt.Fprint(w, `{"disks": []}`)
				}).Methods(http.MethodGet)

				return r
			},
			wantErr: ErrNotFound,
		},
		{
			name: "error: disk size is not same",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks", func(w http.ResponseWriter, r *http.Request) {
					assertTagFilterEqual(t, r.URL.Query())
					fmt.Fprint(w, diskListAPIResponse(pvName, defaultDiskSize, "READY"))
				}).Methods(http.MethodGet)

				return r
			},
			capacityBytes: util.GiBToBytes(200),
			wantErr:       ErrDiskExistsDiffSize,
		},
		{
			name: "error: server is in maintenance",
			mockServerHandler: func(t *testing.T) http.Handler {
				return maintenanceResponse()
			},
			wantErr: fmt.Errorf("could not list the disks: failed list disks: [503 Service Unavailable] Server.Maintenance: The system is in maintenance now."), // nolint: revive
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				client: createClientForTesting(ts.URL),
			}
			got, err := c.GetDiskByName(context.Background(), pvName, tt.capacityBytes)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetDiskByID(t *testing.T) {
	diskName := uuid.New().String()

	tests := []struct {
		name              string
		mockServerHandler func(t *testing.T) http.Handler
		want              *Volume
		wantErr           error
	}{
		{
			name: "success: get disk by ID",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, diskAPIResponse(diskName, defaultDiskSize, "READY"))
				}).Methods(http.MethodGet)

				return r
			},
			want: &Volume{
				VolumeID:         diskName,
				CapacityGiB:      defaultDiskSize,
				AvailabilityZone: defaultZone,
			},
		},
		{
			name: "error: disk is not found",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/disks/{name}", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, errorResponse(
						"Client.InvalidParameterNotFound.Disk",
						fmt.Sprintf("The disk '%s' could not be found.", diskName)),
					)
				}).Methods(http.MethodGet)

				return r
			},
			wantErr: ErrNotFound,
		},
		{
			name: "error: server is in maintenance",
			mockServerHandler: func(t *testing.T) http.Handler {
				return maintenanceResponse()
			},
			wantErr: fmt.Errorf("failed get disk: [503 Service Unavailable] Server.Maintenance: The system is in maintenance now."), // nolint: revive
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				client: createClientForTesting(ts.URL),
			}
			got, err := c.GetDiskByID(context.Background(), diskName)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsExistNode(t *testing.T) {
	tests := []struct {
		name              string
		mockServerHandler func(t *testing.T) http.Handler
		want              bool
	}{
		{
			name: "return true if node exists",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/clusters/{name}", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(w, `{"cluster": {"nodePools": [{"nodes": [{"name": "%s"}]}]}}`, testNodeName)
				}).Methods(http.MethodGet)

				return r
			},
			want: true,
		},
		{
			name: "return false if node does not exist",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/clusters/{name}", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, `{"cluster": {"nodePools": [{"nodes": [{"name": "test-node-02"}]}]}}`)
				}).Methods(http.MethodGet)

				return r
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				sdk: createSDKClientForTesting(ts.URL),
			}
			got := c.IsExistNode(context.Background(), testClusterName, testNodeName)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetClusterName(t *testing.T) {
	nrn := "nrn:nifcloud:hatoba:%s:USERID:cluster:test-cluster-id"

	tests := []struct {
		name              string
		mockServerHandler func(t *testing.T) http.Handler
		want              string
		wantErr           error
	}{
		{
			name: "success: get cluster name from NRN",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/clusters/{nrn}", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(w, `{"cluster": {"name": "%s", "nrn": "%s"}}`, testClusterName, nrn)
				}).Methods(http.MethodGet)

				return r
			},
			want: testClusterName,
		},
		{
			name: "error: specified cluster is not found",
			mockServerHandler: func(t *testing.T) http.Handler {
				r := mux.NewRouter()

				r.HandleFunc("/v1/clusters/{nrn}", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, errorResponse(
						"Client.InvalidParameterNotFound.Cluster",
						fmt.Sprintf("The cluster '%s' could not be found.", testClusterName)),
					)
				}).Methods(http.MethodGet)

				return r
			},
			want:    testClusterName,
			wantErr: ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.mockServerHandler(t))
			defer ts.Close()

			c := &cloud{
				sdk: createSDKClientForTesting(ts.URL),
			}
			got, err := c.GetClusterName(context.Background(), nrn)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
