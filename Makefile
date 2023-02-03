.PHONY: test
test:
	go test -cover `go list ./...`

.PHONY: generate-mock
generate-mock:
	mockgen -source ./api/v2/handler.go -destination ./api/v2/mock/handler_mock.go -package mock
	mockgen -source ./api/v1/jobs/job_handler.go -destination ./api/v1/jobs/mock/job_mock.go -package mock
	mockgen -source ./api/v1/batches/batch_handler.go -destination ./api/v1/batches/mock/batch_mock.go -package mock

.HONY: staticcheck
staticcheck:
	staticcheck `go list ./...` && go vet `go list ./...`