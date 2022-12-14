.PHONY: test
test:
	go test -cover `go list ./...`

.PHONY: generate-mock
generate-mock:
	mockgen -source ./api/jobs/job_handler.go -destination ./api/jobs/mock/job_mock.go -package mock
	mockgen -source ./api/batches/batch_handler.go -destination ./api/batches/mock/batch_mock.go -package mock

.HONY: staticcheck
staticcheck:
	staticcheck `go list ./...` && go vet `go list ./...`