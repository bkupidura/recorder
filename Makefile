# Run tests
.PHONY: test
test:
	go test -cover -covermode atomic ./...
	go test -cover -tags=integration .

# Format code
.PHONY: vet
vet:
	go fmt ./...
	go vet ./...
	go mod tidy
