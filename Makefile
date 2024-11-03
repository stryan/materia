all: build


build: generate build-materia


generate:
	go generate ./...

build-materia:
	go build -o materia ./cmd/materia

lint:
	golangci-lint run ./cmd/... ./internal/...
tools:
	go install golang.org/x/tools/cmd/stringer@latest
clean:
	rm materia

.PHONY: clean tools all
