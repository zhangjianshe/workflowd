Workflow Daemon (workflowd) using gRPC Streaming
This project demonstrates bi-directional streaming communication between a Kubelet-style agent (workflowd) and a control plane (APIServer Mock) using gRPC over HTTP/2.

1. Protocol Buffer Compilation
   The Go files for the gRPC client/server must be generated from the .proto file.

A. Install Protobuf Tools
You must have protoc installed, as well as the Go plugins:

# Install the protoc compiler (OS-dependent, usually via package manager)
# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

B. Generate Go Files
Create a Makefile in the root of the workflowd directory with the following content:

all: proto

proto: proto/workflow.proto
protoc --go_out=./ --go_opt=paths=source_relative \
--go-grpc_out=./ --go-grpc_opt=paths=source_relative \
proto/workflow.proto

Then, run the following command:

make proto

This will generate proto/workflow.pb.go and proto/workflow_grpc.pb.go.

2. Running the Example
   A. Start the Mock APIServer (Server)
   The server sends the initial workflow command.

go run server_mock.go
# Server will start on port 50051

B. Start the Workflow Daemon (Client)
The daemon connects, executes the workflow, and reports status updates.

go run main.go

The client will connect, receive the START command, execute the dummy shell script, stream status updates, and then disconnect.