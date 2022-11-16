package driver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

const (
	// DriverName is name for this CSI
	DriverName = "disk.csi.hatoba.nifcloud.com"
	// TopologyKey is key
	TopologyKey = "topology." + DriverName + "/zone"
)

// Mode is the operating mode of the CSI driver.
type Mode string

const (
	// ControllerMode is the mode that only starts the controller service.
	ControllerMode Mode = "controller"
	// NodeMode is the mode that only starts the node service.
	NodeMode Mode = "node"
	// AllMode is the mode that only starts both the controller and the node service.
	AllMode Mode = "all"
)

// Driver is CSI driver object
type Driver struct {
	controllerService
	nodeService

	srv     *grpc.Server
	options *Options
}

// Options is option for CSI driver.
type Options struct {
	endpoint string
	mode     Mode
}

// NewDriver creates the new CSI driver
func NewDriver(options ...func(*Options)) (*Driver, error) {
	klog.Infof("Driver: %v Version: %v", DriverName, driverVersion)

	driverOptions := Options{
		endpoint: DefaultCSIEndpoint,
	}
	for _, option := range options {
		option(&driverOptions)
	}

	driver := Driver{
		options: &driverOptions,
	}

	switch driverOptions.mode {
	case ControllerMode:
		controller, err := newControllerService()
		if err != nil {
			return nil, fmt.Errorf("failed to create controller service: %s", err)
		}
		driver.controllerService = controller
	case NodeMode:
		nodeID := os.Getenv("NODE_NAME")
		if nodeID == "" {
			return nil, fmt.Errorf("NODE_NAME is empty")
		}
		driver.nodeService = newNodeService(nodeID)
	case AllMode:
		nodeID := os.Getenv("NODE_NAME")
		if nodeID == "" {
			return nil, fmt.Errorf("NODE_NAME is empty")
		}
		controller, err := newControllerService()
		if err != nil {
			return nil, fmt.Errorf("failed to create controller service: %s", err)
		}
		driver.controllerService = controller
		driver.nodeService = newNodeService(nodeID)
	default:
		return nil, fmt.Errorf("unknown mode: %s", driverOptions.mode)
	}

	return &driver, nil
}

// Run runs the gRPC server
func (d *Driver) Run() error {
	scheme, addr, err := parseEndpoint(d.options.endpoint)
	if err != nil {
		return err
	}

	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return err
	}

	logErr := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			klog.Errorf("GRPC error: %v", err)
		}
		return resp, err
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logErr),
	}
	d.srv = grpc.NewServer(opts...)

	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterNodeServer(d.srv, d)

	klog.Infof("Listening for connections on address: %#v", listener.Addr())

	return d.srv.Serve(listener)
}

// Stop stops the server
func (d *Driver) Stop() {
	klog.Infof("Stopping server")
	d.srv.Stop()
}

// WithEndpoint sets the endpoint
func WithEndpoint(endpoint string) func(*Options) {
	return func(o *Options) {
		o.endpoint = endpoint
	}
}

// WithMode sets the driver mode
func WithMode(mode Mode) func(*Options) {
	return func(o *Options) {
		o.mode = mode
	}
}

func parseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	addr := filepath.Join(u.Host, filepath.FromSlash(u.Path))

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "tcp":
	case "unix":
		addr = filepath.Join("/", addr)
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix domain socket %q: %v", addr, err)
		}
	default:
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, addr, nil
}
