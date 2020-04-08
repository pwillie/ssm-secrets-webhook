#!/usr/bin/env make

.EXPORT_ALL_VARIABLES:

include Makehelp.mk

CGO_ENABLED ?= 0
GOOS ?= linux
GOARCH ?= amd64

###Build targets
## Build binary
build: build-ssm-env build-ssm-secrets-webhook
build-%: ; $(info $(M) Running build $*...)
	go build -ldflags="-w -s" -o build/$* cmd/$*/*.go

## Run unit tests
test: ; $(info $(M) Running tests...)
	go test ./... -cover -coverprofile unit_cover.out --tags=unit

## Run linting
lint: ; $(info $(M) Running lint...)
	go list ./... | xargs -L1 golint -set_exit_status

## Build local docker image
docker: docker-ssm-env docker-ssm-secrets-webhook
docker-%: ; $(info $(M) Running docker build $*...)
	docker build -f Dockerfile.$* \
		-t $*:local .
