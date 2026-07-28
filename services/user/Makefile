.PHONY: build run test proto clean lint

# Go parameters
GOCMD    := go
GOBUILD  := $(GOCMD) build
GOTEST   := $(GOCMD) test
GOCLEAN  := $(GOCMD) clean
GOMOD    := $(GOCMD) mod
BINARY   := user-server

# Build the server binary.
build:
	$(GOBUILD) -o $(BINARY) ./cmd/server/

# Run the server (reads config from environment).
run: build
	./$(BINARY)

# Run all tests.
test:
	$(GOTEST) -v -race -count=1 ./...

# Generate protobuf code.
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/user.proto

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
