.PHONY: build run test proto tidy fmt clean lint

# Go parameters
GOCMD    := go
GOBUILD  := $(GOCMD) build
GOTEST   := $(GOCMD) test
GOCLEAN  := $(GOCMD) clean
GOMOD    := $(GOCMD) mod
BINARY   := user-server

# Monorepo root (shared proto definitions live in ../../proto).
ROOT := ../..

# Build the server binary.
build:
	$(GOBUILD) -o $(BINARY) ./cmd/server/

# Run the server (reads config from environment).
run: build
	./$(BINARY)

# Run all tests.
test:
	$(GOTEST) -v -race -count=1 ./...

# Generate protobuf into repo-root proto/userpb (source of truth: proto/user.proto).
# Requires protoc-gen-go and protoc-gen-go-grpc on PATH.
proto:
	protoc --go_out=$(ROOT) --go_opt=module=SuIM \
		--go-grpc_out=$(ROOT) --go-grpc_opt=module=SuIM \
		-I $(ROOT)/proto \
		$(ROOT)/proto/user.proto $(ROOT)/proto/file.proto

# Tidy dependencies.
tidy:
	$(GOMOD) tidy

# Format code.
fmt:
	$(GOCMD) fmt ./...

# Run linter (requires golangci-lint).
lint:
	golangci-lint run ./...

# Clean build artifacts.
clean:
	$(GOCLEAN)
	rm -f $(BINARY)
