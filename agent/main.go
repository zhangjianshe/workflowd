package main

import (
	"context"
	"flag" // <-- Added flag package
	"fmt"
	"io"
	"log"
	"os" // <-- Added os package for exiting
	"os/exec"
	_ "strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "workflowd/proto"
)

const (
	serverAddress = "localhost:50051"
	// The client only needs one command to test the stream
	dummyShellCommand = "echo 'Starting workflow execution...'; sleep 2; echo 'Step 1 complete'; sleep 3; echo 'Step 2 complete'; sleep 1; echo 'Workflow finished.'"
)

// These variables are populated at link time by the Makefile's -ldflags.
var (
	Version   string = "dev"
	BuildTime string = "unknown"
)

// Agent represents the workflow daemon client
type Agent struct {
	Client pb.WorkflowServiceClient
	Conn   *grpc.ClientConn
}

func NewAgent(ctx context.Context) *Agent {
	// 1. Establish connection to the server
	conn, err := grpc.DialContext(ctx, serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	return &Agent{
		Client: pb.NewWorkflowServiceClient(conn),
		Conn:   conn,
	}
}

func (a *Agent) Run(ctx context.Context) {
	defer a.Conn.Close()
	log.Printf("Agent connecting to APIServer at %s...", serverAddress)

	// 2. Open a bi-directional stream
	stream, err := a.Client.ExecuteWorkflow(ctx)
	if err != nil {
		log.Fatalf("Could not open stream to APIServer: %v", err)
	}

	log.Println("Successfully established bi-directional stream.")

	// Create channels for concurrent handling of send/receive
	receiveCh := make(chan *pb.Command)
	errorCh := make(chan error)

	// 3. Goroutine to receive commands from the server
	go func() {
		for {
			cmd, err := stream.Recv()
			if err == io.EOF {
				log.Println("Server closed the command stream.")
				close(receiveCh)
				return
			}
			if err != nil {
				errorCh <- fmt.Errorf("error receiving command: %v", err)
				return
			}
			receiveCh <- cmd
		}
	}()

	// 4. Main loop to process received commands
	for {
		select {
		case cmd, ok := <-receiveCh:
			if !ok {
				// Receive channel closed, stream finished
				return
			}
			// Corrected to use GetTaskId()
			log.Printf("--> RECEIVED COMMAND: %s (ID: %s)", cmd.GetType(), cmd.GetTaskId())
			if cmd.GetType() == pb.Command_START {
				// Execute the received workflow command concurrently
				// Corrected to pass TaskId
				go a.executeAndReport(stream, cmd.GetTaskId())
			}
		case err := <-errorCh:
			log.Printf("Stream error detected: %v", err)
			return
		case <-ctx.Done():
			log.Println("Client context cancelled. Closing stream.")
			return
		}
	}
}

// executeAndReport simulates running a shell command and streaming status updates back to the server
// Renamed parameter to taskId for clarity
func (a *Agent) executeAndReport(stream pb.WorkflowService_ExecuteWorkflowClient, taskId string) {
	log.Printf("Executing Workflow %s...", taskId)

	// Start with PENDING status
	a.sendStatus(stream, taskId, pb.Status_PENDING, "Workflow received and starting...")

	cmd := exec.Command("sh", "-c", dummyShellCommand)

	// Pipe command output to stream
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error setting up stdout pipe for Workflow %s: %v", taskId, err)
		a.sendStatus(stream, taskId, pb.Status_FAILED, fmt.Sprintf("Setup error: %v", err))
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("Error starting command for Workflow %s: %v", taskId, err)
		a.sendStatus(stream, taskId, pb.Status_FAILED, fmt.Sprintf("Start error: %v", err))
		return
	}

	a.sendStatus(stream, taskId, pb.Status_RUNNING, "Command process started.")

	// Read and stream progress updates line by line
	// This simulates streaming logs/progress
	buf := make([]byte, 128)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			message := string(buf[:n])
			log.Printf("Workflow %s progress: %s", taskId, message)
			a.sendStatus(stream, taskId, pb.Status_RUNNING, message)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading stdout for Workflow %s: %v", taskId, err)
			a.sendStatus(stream, taskId, pb.Status_FAILED, fmt.Sprintf("Read error: %v", err))
			return
		}
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		log.Printf("Command failed for Workflow %s: %v", taskId, err)
		a.sendStatus(stream, taskId, pb.Status_FAILED, fmt.Sprintf("Execution failed: %v", err))
		return
	}

	// Final success status
	a.sendStatus(stream, taskId, pb.Status_COMPLETED, "Workflow executed successfully.")
}

// Renamed parameter to taskId for clarity
func (a *Agent) sendStatus(stream pb.WorkflowService_ExecuteWorkflowClient, taskId string, state pb.Status_State, message string) {
	status := &pb.Status{
		// Corrected to use TaskId
		TaskId:    taskId,
		Timestamp: time.Now().Unix(),
		State:     state,
		Message:   message,
	}

	if err := stream.Send(status); err != nil {
		log.Printf("Failed to send status for %s: %v", taskId, err)
	}
}

func main() {
	// ----------------------------------------------------
	// Version Flag Handling (Agent)
	// ----------------------------------------------------
	showVersion := flag.Bool("version", false, "Print current version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Agent Version: %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		os.Exit(0)
	}
	// ----------------------------------------------------

	// Ensure the client has time to clean up
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent := NewAgent(ctx)
	agent.Run(ctx)
}
