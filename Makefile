mocks:
	@mkdir -p pkg/mocks
	@counterfeiter -o pkg/mocks/repository.go . pkg/interfaces/repository.go

test:
	@go test ./... -cover
