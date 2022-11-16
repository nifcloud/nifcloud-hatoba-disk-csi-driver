# nifcloud-hatoba-disk-csi-driver

## Overview

The NIFCLOUD Hatoba Disk Container Storage Interface (CSI) Driver provides a CSI interface used by Container Orchestrators to manage the lifecycle of Hatoba Disks.


## Usage

### Deploy CSI Driver

#### Use NIFCLOUD Kubernetes Service Hatoba CSI driver addon (**RECOMMENDED**)

See the [document](https://pfs.nifcloud.com/guide/kubernetes-service-hatoba/disk_csi_driver.htm).

#### Manual Deploy
1. Create NIFCLOUD secret.
    ```sh
    vi deploy/kubernetes/secret.yaml
    kubectl apply -f deploy/kubernetes/secret.yaml
    ```

2. Set the `NIFCLOUD_HATOBA_CLUSTER_NRN` environment variable.
    ```sh
    vi deploy/kubernetes/overlys/stable/controller.yaml
    ```

3. Deploy drivers.
    ```sh
    kubectl apply -k deploy/kubernetes/overlays/stable
    ```

### Create PV

1. Create storage class.
    * `csi.storage.k8s.io/fstype` and `type` is selectable parameter. Please see [CreateVolume Parameters](#createvolume-parameters) section.
    ```yml
    kind: StorageClass
    apiVersion: storage.k8s.io/v1
    metadata:
      name: hatoba-disk-high-speed-flash-a
    provisioner: disk.csi.hatoba.nifcloud.com
    volumeBindingMode: WaitForFirstConsumer
    parameters:
      csi.storage.k8s.io/fstype: ext4
      type: high-speed-flash-a
    allowVolumeExpansion: true
    ```


2. Create PVC and Pod.
    ```yml
    apiVersion: apps/v1
    kind: StatefulSet
    metadata:
      name: web
    spec:
      serviceName: "nginx"
      replicas: 2
      selector:
        matchLabels:
          app: nginx
      template:
        metadata:
          labels:
            app: nginx
        spec:
          containers:
          - name: nginx
            image: k8s.gcr.io/nginx-slim:0.8
            ports:
            - containerPort: 80
              name: web
            volumeMounts:
            - name: www
              mountPath: /usr/share/nginx/html
      volumeClaimTemplates:
      - metadata:
          name: www
        spec:
          accessModes: [ "ReadWriteOnce" ]
          storageClassName: hatoba-disk-high-speed-flash-a
          resources:
            requests:
              storage: 10Gi
    ```

3. Driver will craete new disk and attach it to node.
    ```sh
    kubectl get pvc
    kubectl get pv
    kubectl get pods
    ```


## CreateVolume Parameters

There are several optional parameters that could be passes into CreateVolumeRequest.parameters map:

### csi.storage.k8s.io/fsType
#### description

File system type that will be formatted during volume creation

#### values

* xfs
* ext2
* ext3
* ext4

#### default

ext4

### type
#### description

Storage type (See https://pfs.nifcloud.com/service/disk.htm)

#### values

* standard-flash-a
* standard-flash-b
* standard-flash (randomly select a or b)
* high-speed-flash-a
* high-speed-flash-b
* high-speed-flash (randomly select a or b)

#### default

standard-flash-a

