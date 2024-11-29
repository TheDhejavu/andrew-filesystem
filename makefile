# Go parameters
BINARY_NAME=afs
PROTO_DIR=internal/proto
PROTO_GEN_DIR=internal/proto/gen 
GO_MODULE_NAME=github.com/TheDhejavu/afs-protocol

# Tools
PROTOC=protoc
GO=go
GOFMT=gofmt

# Install protoc plugins
.PHONY: install-tools
install-tools:
	@echo "Installing protoc-gen-go..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@echo "Installing protoc-gen-go-grpc..."
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate protobuf and gRPC code
.PHONY: proto
proto:
	@echo "Creating directory $(PROTO_GEN_DIR)"
	mkdir -p $(PROTO_GEN_DIR)
	
	@echo "Generating protobuf code..."
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=. \
		--go_opt=module=$(GO_MODULE_NAME) \
		--go-grpc_out=. \
		--go-grpc_opt=module=$(GO_MODULE_NAME) \
		$(PROTO_DIR)/*.proto


# Clean proto generated files
.PHONY: clean-proto
clean-proto:
	@echo "Cleaning generated proto files..."
	@rm -rf $(PROTO_GEN_DIR)/*


# Build server and client
.PHONY: build
build:
	@echo "Building server..."
	$(GO) build -o bin/afs-server cmd/afs-server/main.go
	@echo "Building client..."
	$(GO) build -o bin/afs-client cmd/afs-client/main.go

# Run server
.PHONY: run-server
run-server:
	./bin/afs-server

# Run client
.PHONY: run-client
run-client:
	./bin/afs-client

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) -w .

# Run tests
.PHONY: test
test:
	$(GO) test ./...


# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f bin/*
	rm -f $(PROTO_DIR)/*.pb.go

# Install dependencies
.PHONY: deps
deps:
	$(GO) mod tidy

# All-in-one setup command
.PHONY: setup
setup: install-tools deps proto fmt build

# Development workflow
.PHONY: dev
dev: fmt test build

# Help command
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make setup      - Complete setup (tools, init, deps, proto, build)"
	@echo "  make proto      - Generate protobuf and gRPC code"
	@echo "  make build      - Build server and client"
	@echo "  make run-server - Run the server"
	@echo "  make run-client - Run the client"
	@echo "  make test       - Run tests"
	@echo "  make fmt        - Format code"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make deps       - Install dependencies"
	@echo "  make dev        - Development workflow (fmt, lint, test, build)"