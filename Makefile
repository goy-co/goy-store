.PHONY: all proto rust go test test-all e2e-up e2e-down test-e2e-rust test-e2e-go test-e2e clean fmt lint

all: proto rust go

# Generate Protobuf code for both Rust and Go
proto:
	@echo "Generating Protobuf code..."
	@mkdir -p rust/src/proto
	@mkdir -p go/pb
	# Note: In a real environment, you would run protoc with the appropriate plugins
	# For Rust: protoc --rust_out=rust/src/proto proto/store.proto
	# For Go: protoc --go_out=go/pb --go-grpc_out=go/pb proto/store.proto
	@echo "Protobuf generation placeholder. Run actual protoc commands in CI/CD."

# Build Rust crate
rust:
	@echo "Building Rust crate..."
	cd rust && cargo build

# Build Go package
go:
	@echo "Building Go package..."
	cd go && go build ./...

# Run unit tests for both Rust and Go
test:
	@echo "Running Rust unit tests..."
	cd rust && cargo test
	@echo "Running Go unit tests..."
	cd go && go test ./...

# Infrastructure for E2E tests
e2e-up:
	@echo "Starting E2E test containers..."
	docker compose up -d --wait

e2e-down:
	@echo "Stopping E2E test containers..."
	docker compose down -v

# E2E Tests
test-e2e-rust:
	@echo "Running Rust E2E integration tests..."
	cd rust && cargo test --test e2e --all-features

test-e2e-go:
	@echo "Running Go E2E integration tests..."
	cd go && go test -tags=e2e -v ./e2e/...

test-e2e: e2e-up
	@echo "Running all E2E integration tests..."
	@$(MAKE) test-e2e-rust
	@$(MAKE) test-e2e-go
	@$(MAKE) e2e-down

# Run all tests (unit and e2e)
test-all: test test-e2e

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd rust && cargo clean
	cd go && go clean

# Format code
fmt:
	@echo "Formatting Rust code..."
	cd rust && cargo fmt
	@echo "Formatting Go code..."
	cd go && go fmt ./...

# Lint code
lint:
	@echo "Linting Rust code..."
	cd rust && cargo clippy
	@echo "Linting Go code..."
	cd go && go vet ./...