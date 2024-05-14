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
	docker build -t $(DOCKER_REGISTRY)/radix-job-scheduler:$(TAG) -f Dockerfile .

.PHONY: docker-push
docker-push:
	az acr login --name $(CONTAINER_REPO)
	docker push $(DOCKER_REGISTRY)/radix-job-scheduler:$(TAG)

.PHONY: deploy
deploy: docker-build docker-push

.PHONY: docker-push-main
docker-push-main:
	docker build -t $(DOCKER_REGISTRY)/radix-job-scheduler:main-latest -f Dockerfile .
	az acr login --name $(CONTAINER_REPO)
	docker push $(DOCKER_REGISTRY)/radix-job-scheduler:main-latest

.PHONY: test
test:
	go test -cover `go list ./...`

lint: bootstrap
	golangci-lint run --timeout=30m --max-same-issues=0

.PHONY: mocks
mocks: bootstrap
	mockgen -source ./api/v2/handler.go -destination ./api/v2/mock/handler_mock.go -package mock
	mockgen -source ./api/v1/jobs/job_handler.go -destination ./api/v1/jobs/mock/job_mock.go -package mock
	mockgen -source ./api/v1/batches/batch_handler.go -destination ./api/v1/batches/mock/batch_mock.go -package mock
	mockgen -source ./models/notifications/notifier.go -destination ./models/notifications/notifier_mock.go -package notifications

.PHONY: generate
generate: swagger mocks

.PHONY: verify-generate
verify-generate: generate
	git diff --exit-code

HAS_SWAGGER       := $(shell command -v swagger;)
HAS_GOLANGCI_LINT := $(shell command -v golangci-lint;)
HAS_MOCKGEN       := $(shell command -v mockgen;)


bootstrap:
ifndef HAS_SWAGGER
	go install github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0
endif
ifndef HAS_GOLANGCI_LINT
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2
endif
ifndef HAS_MOCKGEN
	go install github.com/golang/mock/mockgen@v1.6.0
endif
