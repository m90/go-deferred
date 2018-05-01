default: vet test

test:
	@go test -v -race -cover ./...

vet:
	@go vet ./...

.PHONY: test vet
