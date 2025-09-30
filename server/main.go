package main

import (
	"flag" // <-- Added flag package
	"fmt"
	"io"
	"log"
	"net"
	"os" // <-- Added os package for exiting
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer" // <-- Required for retrieving client address
	pb "workflowd/proto"
)

const (
	port = ":50051"
)

// These variables are populated at link time by the Makefile's -ldflags.
var (
	Version   string = "dev"
	BuildTime string = "unknown"
)

// server implements the WorkflowServiceServer interface
type server struct {
	pb.UnimplementedWorkflowServiceServer
	activeStreams map[string]chan *pb.Command
	mu            sync.Mutex
}

func newServer() *server {
	return &server{
		activeStreams: make(map[string]chan *pb.Command),
	}
}

// ExecuteWorkflow is the core bi-directional streaming RPC.
func (s *server) ExecuteWorkflow(stream pb.WorkflowService_ExecuteWorkflowServer) error {
	ctx := stream.Context()

	// Retrieve client connection address using gRPC peer context
	var clientAddr string
	p, ok := peer.FromContext(ctx)
	if ok {
		clientAddr = p.Addr.String()
	} else {
		clientAddr = "unknown peer"
	}

	// 1. Setup Connection and Channel
	connID := fmt.Sprintf("Conn-%d", time.Now().UnixNano())
	// Log connection with client address
	log.Printf("[%s] New agent connected from: %s", connID, clientAddr)

	// Create an unbuffered channel to queue commands for this specific agent
	commandQueue := make(chan *pb.Command)

	s.mu.Lock()
	s.activeStreams[connID] = commandQueue
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.activeStreams, connID)
		s.mu.Unlock()
		close(commandQueue) // Close the channel to signal the sender goroutine to exit
		log.Printf("[%s] Agent disconnected. Total active agents: %d", connID, len(s.activeStreams))
	}()

	// 2. Start a Goroutine for Sending Commands (Asynchronous Send)
	sendDone := make(chan struct{})
	go s.sendCommands(connID, stream, commandQueue, sendDone)

	// 3. Initial Command (Sent through the new channel)
	taskID := fmt.Sprintf("TASK-%d", time.Now().Unix())
	command := &pb.Command{
		Type:       pb.Command_START,
		TaskId:     taskID,
		Executable: "sh",
		Args:       []string{"-c", "for i in 1 2 3; do echo 'Processing step' $i; sleep 1; done"}, // Shorter job for testing
	}
	log.Printf("[%s] Queueing initial START command for Task %s.", connID, taskID)
	commandQueue <- command

	// 4. Main Loop for Receiving Status Updates
	for {
		select {
		case <-ctx.Done():
			// Client disconnected or server is shutting down
			<-sendDone // Wait for the sender goroutine to finish
			return ctx.Err()
		default:
			status, err := stream.Recv()
			if err == io.EOF {
				// Client closed the stream gracefully
				<-sendDone
				return nil
			}
			if err != nil {
				// Error receiving (e.g., network error)
				<-sendDone
				return err
			}

			log.Printf(">> [Agent %s / Task %s] Status: %s - %s",
				connID, status.TaskId, status.State.String(), status.Message)

			// Logic to handle status updates (simplified for brevity)
			if status.State == pb.Status_COMPLETED && status.TaskId == taskID {
				log.Printf("--- Task %s finished successfully. ---", taskID)
				// Optional: Queue a new command here if needed, or exit the loop
				<-sendDone
				return nil // Task complete, close the stream
			}
		}
	}
}

// Helper function to handle the sending of commands
func (s *server) sendCommands(
	connID string,
	stream pb.WorkflowService_ExecuteWorkflowServer,
	queue <-chan *pb.Command,
	done chan<- struct{},
) {
	defer func() {
		close(done) // Signal that the sending goroutine has finished
		log.Printf("[%s] Command sender goroutine finished.", connID)
	}()

	for cmd := range queue {
		log.Printf("[%s] Sending command %s for Task %s...", connID, cmd.Type.String(), cmd.TaskId)
		if err := stream.Send(cmd); err != nil {
			log.Printf("[%s] Failed to send command: %v. Exiting sender goroutine.", connID, err)
			return // Exit on error
		}
	}
}

// Example function to send a command to a specific agent externally (optional)
func (s *server) SendCommandToAgent(connID string, cmd *pb.Command) bool {
	s.mu.Lock()
	queue, ok := s.activeStreams[connID]
	s.mu.Unlock()

	if !ok {
		log.Printf("Agent %s not found.", connID)
		return false
	}

	// Non-blocking send to the agent's command queue
	select {
	case queue <- cmd:
		return true
	case <-time.After(1 * time.Second):
		log.Printf("Timeout queuing command for agent %s.", connID)
		return false
	}
}

func main() {
	// ----------------------------------------------------
	// Version Flag Handling
	// ----------------------------------------------------
	showVersion := flag.Bool("version", false, "Print current version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("APIServer Version: %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		os.Exit(0)
	}
	// ----------------------------------------------------

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterWorkflowServiceServer(s, newServer())

	log.Printf("Mock APIServer running on %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
