package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/nifcloud/nifcloud-hatoba-disk-csi-driver/pkg/driver"
	"k8s.io/klog"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	var (
		version  bool
		endpoint string
		mode     string
	)

	flag.BoolVar(&version, "version", false, "Print the version and exit.")
	flag.StringVar(&endpoint, "endpoint", driver.DefaultCSIEndpoint, "CSI Endpoint")
	flag.StringVar(&mode, "mode", string(driver.ControllerMode), "Mode")

	klog.InitFlags(nil)
	flag.Parse()

	if version {
		info, err := driver.GetVersionJSON()
		if err != nil {
			klog.Fatalln(err)
		}
		fmt.Println(info)
		os.Exit(0)
	}

	drv, err := driver.NewDriver(
		driver.WithEndpoint(endpoint),
		driver.WithMode(driver.Mode(mode)),
	)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}
