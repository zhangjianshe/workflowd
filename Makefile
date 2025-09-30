BUILD_TIME := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_TAG := $(shell git describe --tags --always --dirty)
GIT_HASH := $(shell git rev-parse --short HEAD)
VERSION := $(GIT_TAG)-$(GIT_HASH)

.PHONY: all proto clean modules agent server release

all: proto modules agent server dist

modules:
	go mod tidy
	go mod verify

proto: proto/workflow.proto
	@echo "Generating protobuf Go source files..."
	protoc --go_out=./ --go_opt=paths=source_relative \
    --go-grpc_out=./ --go-grpc_opt=paths=source_relative \
    --go_opt=Mproto/workflow.proto=workflowd/proto \
    --go-grpc_opt=Mproto/workflow.proto=workflowd/proto \
    proto/workflow.proto

	@echo "Protobuf generation complete."

clean:
	@echo "Cleaning generated protobuf files and executables..."
	rm -rf ./dist
	rm -f dist proto/workflow.pb.go proto/workflow_grpc.pb.go wf-agent wf-server

agent: ./agent/main.go
	go build -mod=mod -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)" -o wf-agent ./agent

server: ./server/main.go
	go build -mod=mod -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)" -o wf-server ./server

dist: agent server
	mkdir -p dist
	cp wf-server dist/
	cp wf-agent  dist/

release: clean proto modules
	@echo "Building wf-agent in release mode (stripped symbols)..."
	go build -mod=mod -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)" -o wf-agent-release ./agent

	@echo "Building wf-server in release mode (stripped symbols)..."
	go build -mod=mod -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)" -o wf-server-release ./server/main.go

	@echo "Packaging release binaries..."
	@mkdir -p dist

	@cp wf-agent-release dist/wf-agent
	@cp wf-server-release dist/wf-server
	@rm -f wf-agent-release wf-server-release

	@echo "=========================================================="
	@echo "Release build successful! Optimized executables are in the <dist> folder."
	@echo "VERSION: $(VERSION)"
	@echo "Server size: $$(du -h dist/wf-server | awk '{print $$1}')"
	@echo "Agent size: $$(du -h dist/wf-agent | awk '{print $$1}')"
	@echo "=========================================================="