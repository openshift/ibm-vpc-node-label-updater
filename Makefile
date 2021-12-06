SHELL = /bin/bash
EXE_WATCHER_NAME=vpc-node-label-updater
WATCHER_NAME=vpcNodeLabelUpdater
IMAGE = ibm/${EXE_WATCHER_NAME}
GOPACKAGES=$(shell go list ./... | grep -v /vendor/ | grep -v /cmd | grep -v /tests)
VERSION := latest

GIT_COMMIT_SHA="$(shell git rev-parse HEAD 2>/dev/null)"
GIT_REMOTE_URL="$(shell git config --get remote.origin.url 2>/dev/null)"
BUILD_DATE="$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")"
ARCH=$(shell docker version -f {{.Client.Arch}})

# Jenkins vars. Set to `unknown` if the variable is not yet defined
BUILD_NUMBER?=unknown
GO111MODULE_FLAG?=on
export GO111MODULE=$(GO111MODULE_FLAG)

export LINT_VERSION="1.42.1"

COLOR_YELLOW=\033[0;33m
COLOR_RESET=\033[0m

.PHONY: all
all: deps fmt build test buildimage

.PHONY: driver
driver: deps buildimage

.PHONY: deps
deps:
	echo "Installing dependencies ..."
	#glide install --strip-vendor
	go mod download
	go get github.com/pierrre/gotestcover
	@if ! which golangci-lint >/dev/null || [[ "$$(golangci-lint --version)" != *${LINT_VERSION}* ]]; then \
		curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v${LINT_VERSION}; \
	fi

.PHONY: fmt
fmt: lint
	golangci-lint run --disable-all --enable=gofmt --timeout 600s
	@if [ -n "$$(golangci-lint run)" ]; then echo 'Please run ${COLOR_YELLOW}make dofmt${COLOR_RESET} on your code.' && exit 1; fi

.PHONY: dofmt
dofmt:
	golangci-lint run --disable-all --enable=gofmt --fix --timeout 600s

.PHONY: lint
lint:
	golangci-lint run --timeout 600s

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=$(shell go env GOOS) GOARCH=$(shell go env GOARCH) go build -mod=vendor -a -ldflags '-X main.vendorVersion='"${WATCHER_NAME}-${GIT_COMMIT_SHA}"' -extldflags "-static"' -o "${EXE_WATCHER_NAME}" ./cmd/

.PHONY: test
test:
	$(GOPATH)/bin/gotestcover -v -race -short -coverprofile=cover.out ${GOPACKAGES}
	go tool cover -html=cover.out -o=cover.html  # Uncomment this line when UT in place.

.PHONY: ut-coverage
ut-coverage: deps fmt test

.PHONY: buildimage
buildimage: build-systemutil
	docker build	\
        --build-arg git_commit_id=${GIT_COMMIT_SHA} \
        --build-arg git_remote_url=${GIT_REMOTE_URL} \
        --build-arg build_date=${BUILD_DATE} \
        --build-arg BUILD_URL=${BUILD_URL} \
	-t $(IMAGE):$(VERSION)-$(ARCH) -f Dockerfile .
ifeq ($(ARCH), amd64)
	docker tag $(IMAGE):$(VERSION)-$(ARCH) $(IMAGE):$(VERSION)
endif

.PHONY: build-systemutil
build-systemutil:
	docker build --build-arg TAG=$(GIT_COMMIT_SHA) --build-arg OS=linux --build-arg ARCH=$(ARCH) -t node-watcher-builder --pull -f Dockerfile.builder .
	docker run --env GHE_TOKEN=${GHE_TOKEN} --env GOPRIVATE=${GOPRIVATE} node-watcher-builder
	docker cp `docker ps -q -n=1`:/go/bin/${EXE_WATCHER_NAME} ./${EXE_WATCHER_NAME}

.PHONY: clean
clean:
	rm -rf ${EXE_WATCHER_NAME}
	rm -rf $(GOPATH)/bin/${EXE_WATCHER_NAME}
