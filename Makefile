test:
	go test -cover ./...

lint:
	golangci-lint run

.PHONY: test