.PHONY: test
test:
	go test -cover `go list ./...`
