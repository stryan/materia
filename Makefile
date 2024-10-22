BINFILE = materia

SOURCE=$(shell find . -iname "*.go")

$(BINFILE): $(SOURCE)
	@go build -o "$(BINFILE)"

clean:
	rm $(BINFILE)
