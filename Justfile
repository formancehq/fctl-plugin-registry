pre-commit: tidy generate lint validate-local

tidy:
    go mod tidy

generate:
    gofmt -w cmd internal

lint:
    golangci-lint run ./...

tests:
    go test -race -covermode=atomic -coverprofile=coverage.out ./...
    just coverage

build:
    go build -o build/validate ./cmd/validate

validate-local:
    go run ./cmd/validate --offline

validate:
    go run ./cmd/validate

dirty:
    git diff --exit-code

coverage:
    go tool cover -func=coverage.out | awk '/^total:/ { found=1; if (($3 + 0) < 80) { print "Coverage below 80%"; exit 1 } } END { if (!found) exit 1 }'
