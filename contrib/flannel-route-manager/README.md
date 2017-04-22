# flannel-route-manager

The flannel route manager syncs the [flannel](https://github.com/coreos/flannel) routing table to the specified backend.

## Overview

* [Usage](#usage)
* [Backends](#backends)
* [Build](#build)
* [Single Node Demo](#single-node-demo)

## Usage

```
Usage of ./flannel-route-manager:
  -backend="google": backend provider
  -etcd-endpoint="http://127.0.0.1:4001": etcd endpoint
  -etcd-prefix="/coreos.com/network": etcd prefix
  -sync-interval=30: sync interval
```

### Delete all routes

```
$ /opt/bin/flannel-route-manager -delete-all-routes
2014/10/13 07:16:22 deleting all routes
2014/10/13 07:16:23 deleted: flannel-default-10-244-72-0-24
```

### Monitor subnet changes and sync routes

Running the flannel-route-manager service starts a watcher and reconciler goroutine. 

```
flannel-route-manager
2014/10/13 07:17:39 starting fleet route manager...
2014/10/13 07:17:39 reconciler starting...
2014/10/13 07:17:40 reconciler: inserted flannel-default-10-244-72-0-24
2014/10/13 07:17:40 reconciler done
2014/10/13 07:17:47 monitor: deleted flannel-default-10-244-72-0-24
2014/10/13 07:17:52 monitor: inserted flannel-default-10-244-72-0-24
```

> The reconciler interval can be tuned with the `-sync-interval` flag.

## Backends

flannel-route-manager has been designed to support multiple backends, but only ships a single backend today -- the google backend.

### google

The google backend syncs the flannel route table from etcd to GCE for a specific GCE project and network. Currently routes are only created or updated for each subnet managed by flannel.

Route naming scheme:

```
flannel-default-10-0-63-0-24
```

#### Requirements

* [enabled IP forwarding for instances](https://developers.google.com/compute/docs/networking#canipforward) 
* [instance service account](https://developers.google.com/compute/docs/authentication#using)
* [project ID](https://developers.google.com/compute/docs/overview#projectids)

The google backend relies on instance service accounts for authentication. See [Preparing an instance to use service accounts](https://developers.google.com/compute/docs/authentication#using) for more details.

Creating a compute instance with the right permissions and IP forwarding enabled:

```
$ gcloud compute instances create INSTANCE --can-ip-forward --scopes compute-rw
```

## Build

```
mkdir -p "${GOPATH}/src/github.com/kelseyhightower"
cd "${GOPATH}/src/github.com/kelseyhightower"
git clone https://github.com/kelseyhightower/flannel-route-manager.git
cd flannel-route-manager
godep go build .
```

## Single Node Demo

### Create the Compute Instance

The following command will create a GCE instance with flannel, running in subnet allocate only mode, and the flannel-route-manager up and running. 

```
$ gcloud compute instances create flannel-route-manager-test \
--image-project coreos-cloud \
--image coreos-alpha-459-0-0-v20141003 \
--machine-type g1-small \
--can-ip-forward \
--scopes compute-rw \
--metadata-from-file user-data=ext/cloud-config.yaml \
--zone us-central1-a
```

Once the instance is fully booted you should see a new route added under the default network.

### Listing Routes

```
gcloud compute routes list
```
