#!/bin/bash

set -euo pipefail

${GOBIN}/mockgen -package cloud -destination=./pkg/cloud/zz_generated.mock_cloud.go -source pkg/cloud/cloud.go
${GOBIN}/mockgen -package driver -destination=./pkg/driver/zz_generated.mock_mount.go -source pkg/driver/mount.go
