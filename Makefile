PKG=github.com/nifcloud/nifcloud-hatoba-disk-csi-driver
IMAGE?=ghcr.io/nifcloud/nifcloud-hatoba-disk-csi-driver
VERSION?=$(shell git describe --tags --match="v*" --abbrev=14 HEAD)
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS?="-X $(PKG)/pkg/driver.driverVersion=$(VERSION) -X $(PKG)/pkg/driver.gitCommit=$(GIT_COMMIT) -X $(PKG)/pkg/driver.buildDate=$(BUILD_DATE) -s -w"

ifndef GOBIN
GOBIN=$(shell pwd)/bin
endif

TOOLS=\
	github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1 \
	github.com/golang/mock/mockgen@v1.6.0 \
	golang.org/x/tools/cmd/goimports@latest \
	github.com/google/go-licenses@latest

.PHONY: build
build:
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux go build -ldflags $(LDFLAGS) -o bin/nifcloud-hatoba-disk-csi-driver ./cmd/

.PHONY: test
test:
	@go test -cover ./...

lint:
	@$(GOBIN)/golangci-lint run

.PHONY: fmt
fmt:
	@$(GOBIN)/goimports -w $$(find . -type f -name '*.go')

.PHONY: image
image:
	@docker build -t $(IMAGE):$(VERSION) .

.PHONY: push
push:
	@docker push $(IMAGE):$(VERSION)

.PHONY: license-check
license-check:
	@$(GOBIN)/go-licenses check --ignore $(PKG) ./...

.PHONY: update-gomock
update-gomock:
	@GOBIN=$(GOBIN) ./hack/update-gomock.sh
	@$(MAKE) fmt

.PHONY: install-tools
install-tools:
	@mkdir -p $(GOBIN)
	@for tool in $(TOOLS); do \
		GOBIN=$(GOBIN) go install $$tool; \
	done
