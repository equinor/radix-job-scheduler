.PHONY: test
test:
	go test -cover `go list ./...`

.PHONY: generate-mock
generate-mock:
	mockgen -source ./api/jobs/job.go -destination ./api/jobs/mock/job_mock.go -package mock
	mockgen -source ./api/batches/batch.go -destination ./api/batches/mock/batch_mock.go -package mock