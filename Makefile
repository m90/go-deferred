default: test

test:
	@go test -v -race -cover ./...

.PHONY: test
