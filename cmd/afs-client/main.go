package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheDhejavu/afs-protocol/internal/client"
	"github.com/spf13/cobra"
)

var (
	serverAddr string
	mountPath  string
	clientID   string
	rootCmd    = &cobra.Command{
		Use:   "afs-client",
		Short: "Andrew Distributed File System Client",
	}
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "localhost:50051", "server address")
	rootCmd.PersistentFlags().StringVar(&clientID, "id", "", "client identifier")
	rootCmd.PersistentFlags().StringVar(&mountPath, "mount", "./tmp/client", "Storage directory for client cache")
	rootCmd.MarkPersistentFlagRequired("id")

	// Add commands
	rootCmd.AddCommand(storeCmd)
	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(watchCmd)
}

var storeCmd = &cobra.Command{
	Use:   "store [file]",
	Short: "Store a file in the system",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filepath := args[0]
		cli, err := client.NewClient(serverAddr, clientID, mountPath)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}
		defer cli.Close()

		if err := cli.Store(cmd.Context(), filepath); err != nil {
			return fmt.Errorf("failed to store file: %v", err)
		}
		fmt.Printf("Successfully stored %s\n", filepath)
		return nil
	},
}

var fetchCmd = &cobra.Command{
	Use:   "fetch [file]",
	Short: "Fetch a file from the system",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]
		cli, err := client.NewClient(serverAddr, clientID, mountPath)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}
		defer cli.Close()

		if err := cli.Fetch(cmd.Context(), filename); err != nil {
			return fmt.Errorf("failed to fetch file: %v", err)
		}
		fmt.Printf("Successfully fetched %s\n", filename)
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete [file]",
	Short: "Delete a file from the system",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]
		cli, err := client.NewClient(serverAddr, clientID, mountPath)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}
		defer cli.Close()

		if err := cli.Delete(cmd.Context(), filename); err != nil {
			return fmt.Errorf("failed to fetch file: %v", err)
		}
		fmt.Printf("Successfully deleted %s\n", filename)
		return nil
	},
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch the mount directory for changes and sync with AFS",
	RunE: func(cmd *cobra.Command, args []string) error {
		cli, err := client.NewClient(serverAddr, clientID, mountPath)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}
		defer cli.Close()

		coord := client.NewCoordinator()

		watcher, err := client.NewAFSWatcher(cli, coord)
		if err != nil {
			return fmt.Errorf("failed to create watcher: %v", err)
		}
		defer watcher.Close()

		// Start watching the mount path
		if err := watcher.Watch(mountPath); err != nil {
			return fmt.Errorf("failed to start watching directory: %v", err)
		}

		// Start the recursive sync watcher
		fileSync := client.NewFileSync(cli, coord)
		fileSync.Start(cmd.Context())

		fmt.Printf("Watching mount directory: %s\n", mountPath)
		fmt.Println("Press Ctrl+C to stop...")

		// Wait for interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\nStopping watcher...")
		return nil
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
