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

##@ Test

.PHONY: e2e-test
e2e-test: build
	go run main.go --output-dir testdata testdata 
