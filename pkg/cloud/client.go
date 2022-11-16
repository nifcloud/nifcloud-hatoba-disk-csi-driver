package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"k8s.io/klog"
)

type ListDisksResponse struct {
	Disks []*Disk `json:"disks"`
}

type GetDiskRequest struct {
	DiskName string
}

type GetDiskResponse struct {
	Disk *Disk `json:"disk"`
}

type CreateDiskRequest struct {
	Disk *DiskToCreate `json:"disk"`
}

type DiskToCreate struct {
	Name             string          `json:"name"`
	Cluster          *ClusterForDisk `json:"cluster"`
	Size             uint            `json:"size"`
	Type             string          `json:"type"`
	AvailabilityZone string          `json:"availabilityZone"`
	Description      string          `json:"description"`
	Tags             []*Tag          `json:"tags,omitempty"`
}

type CreateDiskResponse struct {
	Disk *Disk `json:"disk"`
}

type UpdateDiskRequest struct {
	Name string        `json:"-"`
	Disk *DiskToUpdate `json:"disk"`
}

type DiskToUpdate struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Size        uint   `json:"size,omitempty"`
}

type UpdateDiskResponse struct {
	Disk *Disk `json:"disk"`
}

type DeleteDiskRequest struct {
	DiskName string
}

type DeleteDiskResponse struct{}

type AttachDiskRequest struct {
	DiskName string `json:"-"`
	NodeName string `json:"nodeName"`
}

type DetachDiskRequest struct {
	DiskName string `json:"-"`
	NodeName string `json:"nodeName"`
}

type Disk struct {
	Name             string            `json:"name"`
	Cluster          *ClusterForDisk   `json:"cluster,omitempty"`
	Size             uint              `json:"size"`
	Type             string            `json:"type"`
	AvailabilityZone string            `json:"availabilityZone"`
	Attachments      []*DiskAttachment `json:"attachments,omitempty"`
	Description      string            `json:"description"`
	Status           string            `json:"status"`
	NRN              string            `json:"nrn"`
}

type ClusterForDisk struct {
	Name string `json:"name"`
}

type DiskAttachment struct {
	NodeName   string `json:"nodeName"`
	DevicePath string `json:"devicePath"`
	AttachTime string `json:"attachTime"`
	Status     string `json:"status"`
}

type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type client struct {
	region          string
	endpoint        string
	accessKeyID     string
	secretAccessKey string
}

func newClient(region, accessKeyID, secretAccessKey, endpoint string) *client {
	return &client{
		region:          region,
		endpoint:        endpoint,
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
	}
}

func (c *client) request(method, path string, body io.ReadSeeker) ([]byte, error) {
	req, err := http.NewRequest(method, c.endpoint+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")

	signer := v4.NewSigner(
		credentials.NewStaticCredentials(c.accessKeyID, c.secretAccessKey, ""),
	)
	_, err = signer.Sign(req, body, "hatoba", c.region, time.Now())
	if err != nil {
		return nil, err
	}

	dumpreq, _ := httputil.DumpRequest(req, true)
	klog.V(4).Infof("request: %s", string(dumpreq))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	dumpres, _ := httputil.DumpResponse(resp, true)
	klog.V(4).Infof("response: %s", string(dumpres))

	if resp.StatusCode >= 400 {
		errorBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("could not read the error response body: %w", err)
		}

		errorResponse := &ErrorResponse{}
		if err := json.Unmarshal(errorBody, &errorResponse); err != nil {
			return nil, fmt.Errorf("could not parse the error response: %w", err)
		}

		return nil, fmt.Errorf("[%s] %s: %s", resp.Status, errorResponse.Code, errorResponse.Message)
	}

	return io.ReadAll(resp.Body)
}

func (c *client) ListDisks(filters string) (*ListDisksResponse, error) {
	path := "/v1/disks"
	if filters != "" {
		path = path + "?filters=" + url.QueryEscape(filters)
	}
	raw, err := c.request(http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed list disks: %w", err)
	}

	res := &ListDisksResponse{}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (c *client) GetDisk(req *GetDiskRequest) (*GetDiskResponse, error) {
	raw, err := c.request(http.MethodGet, fmt.Sprintf("/v1/disks/%s", req.DiskName), nil)
	if err != nil {
		return nil, fmt.Errorf("failed get disk: %w", err)
	}

	res := &GetDiskResponse{}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (c *client) CreateDisk(req *CreateDiskRequest) (*CreateDiskResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	raw, err := c.request(http.MethodPost, "/v1/disks", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed create disk: %w", err)
	}

	res := &CreateDiskResponse{}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (c *client) UpdateDisk(req *UpdateDiskRequest) (*UpdateDiskResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	raw, err := c.request(http.MethodPut, fmt.Sprintf("/v1/disks/%s", req.Name), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed update disk: %w", err)
	}

	res := &UpdateDiskResponse{}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (c *client) DeleteDisk(req *DeleteDiskRequest) error {
	_, err := c.request(http.MethodDelete, fmt.Sprintf("/v1/disks/%s", req.DiskName), nil)
	if err != nil {
		return fmt.Errorf("failed delete disk: %w", err)
	}

	return nil
}

func (c *client) AttachDisk(req *AttachDiskRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.request(http.MethodPost, fmt.Sprintf("/v1/disks/%s:attach", req.DiskName), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed attach disk: %w", err)
	}

	return nil
}

func (c *client) DetachDisk(req *DetachDiskRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.request(http.MethodPost, fmt.Sprintf("/v1/disks/%s:detach", req.DiskName), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed detach disk: %w", err)
	}

	return nil
}
