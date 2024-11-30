// cmd/server/main.go
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheDhejavu/afs-protocol/internal/common/storage"
	"github.com/TheDhejavu/afs-protocol/internal/config"
	pb "github.com/TheDhejavu/afs-protocol/internal/proto/gen"
	"github.com/TheDhejavu/afs-protocol/internal/server"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var (
	cfg = &config.Config{}

	rootCmd = &cobra.Command{
		Use:   "afs-server",
		Short: "Andrew Distributed File System Server",
		Long: `A distributed file system server that handles file operations 
               with lock-based concurrency control.`,
		RunE: runServer,
	}
)

func init() {
	rootCmd.Flags().IntVar(&cfg.Port, "port", 50051, "The server port")
	rootCmd.Flags().StringVar(&cfg.MountPath, "mount", "./mount/server",
		"Storage directory for files")
}

func runServer(cmd *cobra.Command, args []string) error {
	// Initialize storage
	storage, err := storage.NewDiskStorage(cfg.MountPath)
	if err != nil {
		return fmt.Errorf("failed to initialize disk storage: %v", err)
	}

	// Create service layer
	fileService := server.NewFileService(storage)

	// Create gRPC handler
	dfsHandler := server.NewDFSHandler(fileService)

	// Initialize gRPC server
	server := grpc.NewServer()
	pb.RegisterFileSystemServiceServer(server, dfsHandler)

	// Start listening
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	// Handle graceful shutdown
	go handleShutdown(server)

	// Start server
	log.Printf("Server starting on port %d", cfg.Port)
	if err := server.Serve(listen); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}

func handleShutdown(server *grpc.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Received shutdown signal, gracefully stopping server...")

	server.GracefulStop()
	log.Println("Server stopped")
	os.Exit(0)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
