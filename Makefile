REPOHOST = ${MATERIA_TEST_REPO_HOST}
REPO = ${MATERIA_TEST_REPO_URL}
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

test-vm: build test test-vm-build test-vm-update
test-vm-build:
	virter vm run --name materia-test-alma-9 --id 101 --wait-ssh alma-9
test-vm-update: build
	virter vm exec materia-test-alma-9 --provision ./scripts/virter_provision.toml --set values.Repo=$(REPO) --set values.RemoteHost=$(REPOHOST)
test-vm-connect:
	virter vm ssh materia-test-alma-9
rm-test-vm:
	virter vm rm materia-test-alma-9

.PHONY: clean tools all test test-vm test-vm-connect rm-test-vm
