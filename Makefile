REPOHOST = ${MATERIA_TEST_REPO_HOST}
REPO = ${MATERIA_TEST_REPO_URL}
all: build


build: generate build-materia


generate:
	go generate ./...

build-materia:
	go build -o materia ./cmd/materia

test: build lint
	age -r age1s2sc3dz0vrcmungswpxwnett7ayvzmfuke8vw0w3te36l742kpqs6kg606 scripts/testrepo/secrets/vault.toml > scripts/testrepo/secrets/vault.age
	age -r age1s2sc3dz0vrcmungswpxwnett7ayvzmfuke8vw0w3te36l742kpqs6kg606 scripts/testrepo/secrets/localhost.toml > scripts/testrepo/secrets/localhost.age
	go test ./scripts/

lint:
	golangci-lint run ./cmd/... ./internal/...
tools:
	go install golang.org/x/tools/cmd/stringer@latest
	go install filippo.io/age/cmd/...@latest
clean:
	rm materia

test-vm: build test test-vm-build test-vm-update
test-vm-build:
	virter vm run --name materia-test-alma-9 --id 101 --wait-ssh alma-9
test-vm-update: build
	virter vm exec materia-test-alma-9 --provision ./scripts/virter_provision.toml --set values.Repo=$(REPO) --set values.RemoteHost=$(REPOHOST)
test-vm-connect:
	virter vm ssh materia-test-alma-9
test-vm-local:
	virter vm run --name materia-local-alma-9 --id 102 --wait-ssh alma-9
	virter vm exec materia-local-alma-9 --provision ./scripts/virter_local_testing.toml --set values.Repo="$MATERIA_LOCAL_REPO_URL" --set vaules.LocalRepo="$MATERIA_LOCAL_REPO"
rm-test-vm:
	virter vm rm materia-test-alma-9

.PHONY: clean tools all test test-vm test-vm-connect rm-test-vm
