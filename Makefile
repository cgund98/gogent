.PHONY: test format lint tidy verify example-simple

test:
	go test ./...

format:
	go fmt ./...

lint:
	golangci-lint run --timeout=5m

tidy:
	go mod tidy

verify:
	go mod verify

example-simple:
	go run ./examples/simple
