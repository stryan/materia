materia:
	go generate ./...
	go build -o materia ./cmd/materia

tools:
	go install golang.org/x/tools/cmd/stringer@latest
clean:
	rm materia
