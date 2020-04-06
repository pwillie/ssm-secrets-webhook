#!/usr/bin/env make

.EXPORT_ALL_VARIABLES:

include Makehelp.mk

CGO_ENABLED ?= 0
COMMIT ?= $(shell git rev-parse --short HEAD)$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GOOS ?= linux
GOARCH ?= amd64
REGISTRY ?= pwillie
VERSION ?= local


###Build targets
## Build binary
build: build-ssm-env build-ssm-secrets-webhook
build-%: ; $(info $(M) Running build $*...)
	go build -o build/$* cmd/$*/*.go

## Run unit tests
test: ; $(info $(M) Running tests...)
	go test ./... -cover -coverprofile unit_cover.out --tags=unit

## Run linting
lint: ; $(info $(M) Running lint...)
	go list ./... | grep -v /vendor/ | xargs -L1 golint -set_exit_status

## Build docker image
docker: docker-ssm-env docker-ssm-secrets-webhook
docker-%: ; $(info $(M) Running docker build $*...)
	docker build -f Dockerfile.$* \
		--build-arg GIT_COMMIT=$(COMMIT) \
		--build-arg VERSION=$(VERSION) \
		-t $(REGISTRY)/$*:$(VERSION) .
