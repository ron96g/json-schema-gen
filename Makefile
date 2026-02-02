.PHONY: all
all: build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

##@ Build

.PHONY: build
build: fmt vet
	go build -o bin/json-schema-gen main.go
