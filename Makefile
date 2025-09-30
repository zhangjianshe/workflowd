BUILD_TIME := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_TAG := $(shell git describe --tags --always)
GIT_HASH := $(shell git rev-parse --short HEAD)
VERSION := $(GIT_TAG)-$(GIT_HASH)
DEB_VERSION := $(shell echo $(VERSION) | sed 's/^v//')

# Configuration for Debian package
PACKAGE_NAME := workflowd
ARCH := amd64
STAGING_DIR := $(CURDIR)/.deb_staging

.PHONY: all proto clean modules agent server release systemd

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

release: clean proto modules systemd
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

systemd:
	@echo "Generating systemd unit file for wf-agent..."
	@mkdir -p dist
	@echo "[Unit]" > dist/wf-agent.service
	@echo "Description=Workflow Agent[cangling.cn] $(VERSION)" >> dist/wf-agent.service
	@echo "After=network.target" >> dist/wf-agent.service
	@echo "" >> dist/wf-agent.service
	@echo "[Service]" >> dist/wf-agent.service
	@echo "Type=simple" >> dist/wf-agent.service
	@echo "User=root" >> dist/wf-agent.service
	@echo "Group=root" >> dist/wf-agent.service
	@echo "WorkingDirectory=/opt/workflowd" >> dist/wf-agent.service
	@echo "ExecStart=/opt/workflowd/wf-agent" >> dist/wf-agent.service
	@echo "Restart=on-failure" >> dist/wf-agent.service
	@echo "StandardOutput=journal" >> dist/wf-agent.service
	@echo "StandardError=journal" >> dist/wf-agent.service
	@echo "" >> dist/wf-agent.service
	@echo "[Install]" >> dist/wf-agent.service
	@echo "WantedBy=multi-user.target" >> dist/wf-agent.service
	@echo "Systemd unit file created at dist/wf-agent.service"

deb: release
	@echo "Building Debian package: $(PACKAGE_NAME)_$(VERSION)-1_$(ARCH).deb"
	
	# 1. Prepare package staging directory structure
	@rm -rf $(STAGING_DIR)
	@mkdir -p $(STAGING_DIR)/DEBIAN
	@mkdir -p $(STAGING_DIR)/opt/workflowd # Install location for binaries
	@mkdir -p $(STAGING_DIR)/lib/systemd/system # Install location for unit file
	
	# 2. Create the control file
	@echo "Package: $(PACKAGE_NAME)" > $(STAGING_DIR)/DEBIAN/control
	@echo "Version: $(DEB_VERSION)-1" >> $(STAGING_DIR)/DEBIAN/control
	@echo "Section: net" >> $(STAGING_DIR)/DEBIAN/control
	@echo "Priority: optional" >> $(STAGING_DIR)/DEBIAN/control
	@echo "Architecture: $(ARCH)" >> $(STAGING_DIR)/DEBIAN/control
	@echo "Maintainer: Zhang JianShe <zhangjianshe@gmail.com>" >> $(STAGING_DIR)/DEBIAN/control
	@echo "Description: Workflow Daemon Server and Agent" >> $(STAGING_DIR)/DEBIAN/control
	@echo " The Workflow Daemon (wf-agent) manages distributed tasks, and wf-agent" >> $(STAGING_DIR)/DEBIAN/control
	@echo " executes them on remote machines." >> $(STAGING_DIR)/DEBIAN/control
	
	# 3. Copy files to the staging directory with their final path structure
	@cp dist/wf-agent $(STAGING_DIR)/opt/workflowd/
	@cp dist/wf-agent $(STAGING_DIR)/opt/workflowd/
	@cp dist/wf-agent.service $(STAGING_DIR)/lib/systemd/system/
	
	# 4. Set permissions (binaries need executable permission)
	@chmod 755 $(STAGING_DIR)/opt/workflowd/wf-agent
	@chmod 755 $(STAGING_DIR)/opt/workflowd/wf-agent
	
	# 5. Build the package
	@dpkg-deb --build $(STAGING_DIR) dist
	
	@rm -rf $(STAGING_DIR)
	@echo "----------------------------------------------------------"
	@echo "Debian package created successfully:"
	@echo "File: dist/$(PACKAGE_NAME)_$(VERSION)-1_$(ARCH).deb"
	@echo "----------------------------------------------------------"
