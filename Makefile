.PHONY: test
test:
	go test -cover `go list ./...`

.PHONY: generate-mock
generate-mock:
	mockgen -source ./api/handlers/job/handler.go -destination ./api/handlers/job/test/handler_mock.go -package mock