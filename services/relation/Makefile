.PHONY: build run test tidy fmt clean

GOCMD    := go
GOBUILD  := $(GOCMD) build
GOTEST   := $(GOCMD) test
GOCLEAN  := $(GOCMD) clean
GOMOD    := $(GOCMD) mod
BINARY   := relation-server

build:
	$(GOBUILD) -o $(BINARY) ./cmd/server/

run: build
	./$(BINARY)

test:
	$(GOTEST) -v -race -count=1 ./...

tidy:
	$(GOMOD) tidy

fmt:
	$(GOCMD) fmt ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY)
