all: build


build: generate build-materia


generate:
	go generate ./...

build-materia:
	go build -o materia ./cmd/materia

test: lint
	age -r age1s2sc3dz0vrcmungswpxwnett7ayvzmfuke8vw0w3te36l742kpqs6kg606 example_repo/vault.toml > example_repo/vault.age
	age -r age1s2sc3dz0vrcmungswpxwnett7ayvzmfuke8vw0w3te36l742kpqs6kg606 example_repo/localhost.toml > example_repo/localhost.age
	go test ./scripts/

lint:
	golangci-lint run ./cmd/... ./internal/...
tools:
	go install golang.org/x/tools/cmd/stringer@latest
clean:
	rm materia

.PHONY: clean tools all test
