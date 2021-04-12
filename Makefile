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

.PHONY: test
test:
	go test -cover `go list ./...`

# This make command is only needed for local testing now
# we also do make swagger inside Dockerfile
.PHONY: swagger
swagger:
	rm -f ./swaggerui_src/swagger.json ./swaggerui/statik.go
	swagger generate spec -o ./swagger.json --scan-models
	mv swagger.json ./swaggerui_src/swagger.json
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

.PHONY: generate-mock
generate-mock:
	mockgen -source ./api/handlers/job/handler.go -destination ./api/handlers/job/test/handler_mock.go -package mock