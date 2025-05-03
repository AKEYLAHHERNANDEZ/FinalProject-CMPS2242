.DEFAULT_GOAL := build

# Configurable variables (can be overridden via command line)
HOST ?= localhost
PORT ?= 4000

CLIENT_SRC := TCPclient.go
SERVER_SRC := TCP.go

CLIENT_BIN := client
SERVER_BIN := server

.PHONY: fmt
fmt:
	@echo "Formatting project..."
	@go fmt ./...

.PHONY: vet
vet: fmt
	@echo "Vetting the project..."
	@go vet ./...

.PHONY: build
build: vet
	@echo "Building server..."
	@go build -o $(SERVER_BIN) $(SERVER_SRC)
	@echo "Building client..."
	@go build -o $(CLIENT_BIN) $(CLIENT_SRC)

.PHONY: run-server
run-server: build
	@echo "Running server on $(HOST):$(PORT)..."
	@./$(SERVER_BIN) -host=$(HOST) -port=$(PORT)

.PHONY: run-client
run-client: build
	@echo "Running client connecting to $(HOST):$(PORT)..."
	@./$(CLIENT_BIN) -host=$(HOST) -port=$(PORT)

.PHONY: clean
clean:
	@echo "Cleaning up binaries..."
	@rm -f $(CLIENT_BIN) $(SERVER_BIN)
