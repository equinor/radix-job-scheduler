ENVIRONMENT ?= dev
VERSION 	?= latest
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
#GIT_TAG		= $(shell git describe --tags --always 2>/dev/null)
#VERSION		?= ${GIT_TAG}

# If you want to escape branch-environment constraint, pass in OVERRIDE_BRANCH=true
ifeq ($(ENVIRONMENT),prod)
	DNS_ZONE = radix.equinor.com
else
	DNS_ZONE = dev.radix.equinor.com
endif

CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io
HASH := $(shell git rev-parse HEAD)
TAG := $(BRANCH)-$(HASH)

echo:
	@echo "ENVIRONMENT : " $(ENVIRONMENT)
	@echo "DNS_ZONE : " $(DNS_ZONE)
	@echo "CONTAINER_REPO : " $(CONTAINER_REPO)
	@echo "DOCKER_REGISTRY : " $(DOCKER_REGISTRY)
	@echo "BRANCH : " $(BRANCH)

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
	docker build -t $(DOCKER_REGISTRY)/radix-job-scheduler:$(VERSION) -t $(DOCKER_REGISTRY)/radix-job-scheduler:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-job-scheduler:$(TAG) -f Dockerfile .

.PHONY: docker-push
docker-push:
	az acr login --name $(CONTAINER_REPO)
	make docker-build
	docker push $(DOCKER_REGISTRY)/radix-job-scheduler:$(BRANCH)-$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-job-scheduler:$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-job-scheduler:$(TAG)

