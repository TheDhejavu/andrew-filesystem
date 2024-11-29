package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheDhejavu/afs-protocol/internal/config"
	pb "github.com/TheDhejavu/afs-protocol/internal/proto/gen"
	"github.com/TheDhejavu/afs-protocol/internal/server"
	"google.golang.org/grpc"
)

func main() {
	cfg := parseFlags()

	// Initialize storage
	storage, err := server.NewDiskStorage(cfg.MountPath)
	if err != nil {
		log.Fatalf("Failed to initialize disk storage: %v", err)
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
		log.Fatalf("Failed to listen: %v", err)
	}

	// Handle graceful shutdown
	go handleShutdown(server)

	// Start server
	log.Printf("Server starting on port %d", cfg.Port)
	if err := server.Serve(listen); err != nil {	
		log.Fatalf("Failed to serve: %v", err)
	}
}

func parseFlags() *config.Config {
	cfg := &config.Config{}

	flag.IntVar(&cfg.Port, "port", 50051, "The server port")
	flag.StringVar(&cfg.MountPath, "mount", "./mnt/server", "Storage directory for files")

	flag.Parse()
	return cfg
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
