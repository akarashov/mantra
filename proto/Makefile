.PHONY: proto generate clean install-tools rebuild check-tools help diagnose

# PROTO_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
PROTO_DIR := .
GO_OUT := ../pkg/grpc
PROTO_FILES := $(wildcard $(PROTO_DIR)/*/*.proto)

help:
	@echo "Available commands:"
	@echo "  make help          - Show this help message"
	@echo "  make proto         - Generate Go code from proto files"
	@echo "  make generate      - Alias for proto"
	@echo "  make clean         - Remove generated files"
	@echo "  make rebuild       - Clean and regenerate files"
	@echo "  make install-tools - Install required tools"
	@echo "  make check-tools   - Check if required tools are installed"
	@echo "  make diagnose      - Print project structure and diagnostics"

check-tools:
	@echo "Checking required tools..."
	@command -v buf >/dev/null 2>&1 || { \
		echo "buf is not installed"; \
		exit 1; \
	}
	@echo "All tools are installed"

install-tools:
	@echo "Installing tools..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@command -v buf >/dev/null 2>&1 || { \
		echo "Installing buf..."; \
		brew install bufbuild/buf/buf 2>/dev/null || \
		go install github.com/bufbuild/buf/cmd/buf@latest; \
	}
	@echo "Tools installed"

proto: check-tools
	@if [ ! -f buf.gen.yaml ]; then \
		echo "buf.gen.yaml not found!"; \
		exit 1; \
	fi
	@if [ -z "$(PROTO_FILES)" ]; then \
		echo "No proto files found in $(PROTO_DIR)"; \
		exit 1; \
	fi
	@echo "Generating golang code from proto files..."
	@echo "   Directory: $(PROTO_DIR)"
	@echo "   Output:    $(GO_OUT)"
	@echo "   Files:     $(notdir $(PROTO_FILES))"
	@buf generate . --template buf.gen.yaml
	@echo "Generation completed"

generate: proto

clean:
	@echo "Cleaning generated files..."
	@if [ -d "$(GO_OUT)" ]; then \
		rm -rf $(GO_OUT)/*.pb.go; \
		rm -rf $(GO_OUT)/*.pb.gw.go 2>/dev/null || true; \
		echo "Clean completed"; \
	else \
		echo "Output directory $(GO_OUT) does not exist"; \
	fi

rebuild: clean proto
	@echo "Rebuild completed"

diagnose:
	@echo "Project structure:"
	@echo "  Current dir: $$PWD"
	@echo "  buf.yaml exists: $$([ -f buf.yaml ] && echo 'OK' || echo 'ERROR')"
	@echo "  buf.gen.yaml exists: $$([ -f buf.gen.yaml ] && echo 'OK' || echo 'ERROR')"
	@echo "  PROTO_DIR exists: $$([ -d $(PROTO_DIR) ] && echo 'OK' || echo 'ERROR')"
	@echo "  Proto files: $(words $(PROTO_FILES)) found"
	@if [ -f buf.yaml ]; then \
		echo "  buf.yaml content:"; \
		cat buf.yaml | sed 's/^/    /'; \
	fi
