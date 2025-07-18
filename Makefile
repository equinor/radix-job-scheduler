ENVIRONMENT ?= dev
VERSION 	?= latest
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
TAG := $(BRANCH)-$(VERSION)

# If you want to escape branch-environment constraint, pass in OVERRIDE_BRANCH=true
ifeq ($(ENVIRONMENT),prod)
	DNS_ZONE = radix.equinor.com
else
	DNS_ZONE = dev.radix.equinor.com
endif

CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io

DOCKER_BUILDX_BUILD_BASE_CMD := docker buildx build -t $(DOCKER_REGISTRY)/radix-job-scheduler:$(TAG) --platform linux/arm64,linux/amd64 -f Dockerfile

echo:
	@echo "ENVIRONMENT : " $(ENVIRONMENT)
	@echo "DNS_ZONE : " $(DNS_ZONE)
	@echo "CONTAINER_REPO : " $(CONTAINER_REPO)
	@echo "DOCKER_REGISTRY : " $(DOCKER_REGISTRY)
	@echo "BRANCH : " $(BRANCH)
	@echo "TAG : " $(TAG)

# This make command is only needed for local testing now
# we also do make swagger inside Dockerfile
.PHONY: swagger
swagger: SHELL:=/bin/bash
swagger: bootstrap
	swagger generate spec -o ./swaggerui/html/swagger.json --scan-models --exclude-deps
	swagger validate ./swaggerui/html/swagger.json

.PHONY: docker-build
docker-build:
	${DOCKER_BUILDX_BUILD_BASE_CMD} .

.PHONY: docker-push
docker-push:
	az acr login --name $(CONTAINER_REPO)
	${DOCKER_BUILDX_BUILD_BASE_CMD} --push .

.PHONY: deploy
deploy: docker-build docker-push

.PHONY: test
test:
	go test -cover `go list ./...`

lint: bootstrap
	golangci-lint run --timeout=30m --max-same-issues=0

.PHONY: mocks
mocks: bootstrap
	mockgen -source ./api/v1/handlers/jobs/job_handler.go -destination ./api/v1/handlers/jobs/mock/job_mock.go -package mock
	mockgen -source ./api/v1/handlers/batches/batch_handler.go -destination ./api/v1/handlers/batches/mock/batch_mock.go -package mock
	mockgen -source ./pkg/notifications/notifier.go -destination ./pkg/notifications/notifier_mock.go -package notifications
	mockgen -source ./pkg/batch/history.go -destination ./pkg/batch/history_mock.go -package batch

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: code-gen
code-gen: bootstrap
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./models/common/..."

.PHONY: generate
generate: tidy code-gen swagger mocks

.PHONY: verify-generate
verify-generate: generate
	git diff --exit-code


HAS_SWAGGER       := $(shell command -v swagger;)
HAS_GOLANGCI_LINT := $(shell command -v golangci-lint;)
HAS_MOCKGEN       := $(shell command -v mockgen;)
HAS_CONTROLLER_GEN := $(shell command -v controller-gen;)

bootstrap:
ifndef HAS_SWAGGER
	go install github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0
endif
ifndef HAS_GOLANGCI_LINT
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.3
endif
ifndef HAS_MOCKGEN
	go install github.com/golang/mock/mockgen@v1.6.0
endif
ifndef HAS_CONTROLLER_GEN
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.2
endif
