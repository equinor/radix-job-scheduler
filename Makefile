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
swagger:
	rm -f ./swaggerui_src/swagger.json ./swaggerui/statik.go
	swagger generate spec -o ./swagger.json --scan-models -x github.com/equinor/radix-job-scheduler/models/v2
	mv swagger.json ./swaggerui_src/swagger.json
	swagger validate ./swaggerui_src/swagger.json && \
	statik -src=./swaggerui_src/ -p swaggerui

.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_REGISTRY)/radix-job-scheduler:$(TAG) -f Dockerfile .

.PHONY: docker-push
docker-push:
	az acr login --name $(CONTAINER_REPO)
	make docker-build
	docker push $(DOCKER_REGISTRY)/radix-job-scheduler:$(TAG)

.PHONY: docker-push-main
docker-push-main:
	docker build -t $(DOCKER_REGISTRY)/radix-job-scheduler:main-latest -f Dockerfile .
	az acr login --name $(CONTAINER_REPO)
	docker push $(DOCKER_REGISTRY)/radix-job-scheduler:main-latest

.PHONY: test
test:
	go test -cover `go list ./...`

.PHONY: mocks
mocks:
	mockgen -source ./api/v2/handler.go -destination ./api/v2/mock/handler_mock.go -package mock
	mockgen -source ./api/v1/jobs/job_handler.go -destination ./api/v1/jobs/mock/job_mock.go -package mock
	mockgen -source ./api/v1/batches/batch_handler.go -destination ./api/v1/batches/mock/batch_mock.go -package mock
	mockgen -source ./models/notifications/notifier.go -destination ./models/notifications/notifier_mock.go -package notifications

.HONY: staticcheck
staticcheck:
	staticcheck `go list ./...` && go vet `go list ./...`
