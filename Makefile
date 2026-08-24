.PHONY: all proto rust go test clean

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

# Run tests for both Rust and Go
test:
	@echo "Running Rust tests..."
	cd rust && cargo test
	@echo "Running Go tests..."
	cd go && go test ./...

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