# Contributing

## Development

```bash
git clone https://github.com/dakaneye/release-pilot.git
cd release-pilot
go build ./cmd/release-pilot/
```

## Commands

```bash
go build ./cmd/release-pilot/    # Build binary
go test ./...                     # Run unit tests
go test ./... -tags acceptance    # Run acceptance tests
go vet ./...                      # Vet
go mod tidy                       # Tidy dependencies
```

## Before Submitting

1. `go build ./cmd/release-pilot/` passes
2. `go vet ./...` passes
3. `go test -race ./...` passes
4. `go mod tidy` produces no changes
5. New functionality has tests

## Pull Requests

- Keep changes focused
- Update tests for new functionality
- Follow existing code style
- Run `gofmt` and `goimports` before committing
