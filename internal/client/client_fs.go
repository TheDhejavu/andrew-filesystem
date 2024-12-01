package client

import (
	"context"
	"fmt"
	"io"

	"github.com/TheDhejavu/afs-protocol/internal/common/channel"
	"github.com/TheDhejavu/afs-protocol/internal/common/storage"
	"github.com/TheDhejavu/afs-protocol/internal/common/types"
	pb "github.com/TheDhejavu/afs-protocol/internal/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	clientID  string
	conn      *grpc.ClientConn
	fsClient  pb.FileSystemServiceClient
	storage   storage.Storage
	mountPath string
}

func NewClient(serverAddr string, clientID string, mountPath string) (*Client, error) {
	// Set up connection with coordinator
	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %v", err)
	}

	diskStorage, err := storage.NewDiskStorage(mountPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize disk storage: %v", err)
	}
	return &Client{
		clientID:  clientID,
		conn:      conn,
		mountPath: mountPath,
		fsClient:  pb.NewFileSystemServiceClient(conn),
		storage:   diskStorage,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) AcquireWriteAccess(ctx context.Context, filename string) error {
	req := &pb.WriteAccessRequest{
		Filename: filename,
		ClientId: c.clientID,
	}

	resp, err := c.fsClient.AcquireWriteAccess(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to acquire write access: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("write access denied: %s", resp.Error)
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, filename string) error {
	req := &pb.DeleteRequest{
		Filename: filename,
		ClientId: c.clientID,
	}

	resp, err := c.fsClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to acquire write access: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("write access denied: %s", resp.Error)
	}

	return nil
}

func (c *Client) GetFileStat(ctx context.Context, filename string) (*types.FileInfo, error) {
	req := &pb.GetFileStatRequest{
		Filename: filename,
	}

	resp, err := c.fsClient.GetFileStat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire write access: %v", err)
	}

	return &types.FileInfo{
		Filename:     resp.Filename,
		ModifiedTime: resp.GetMtime(),
		Size:         resp.Size,
		Checksum:     resp.CrcChecksum,
		IsDeleted:    resp.Deleted,
	}, nil
}

func (c *Client) RequestFilesAsync(ctx context.Context) ([]*types.FileInfo, error) {
	req := &pb.RequestFilesAsyncRequest{
		ClientId: c.clientID,
	}

	resp, err := c.fsClient.RequestFilesAsync(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire write access: %v", err)
	}

	var files = []*types.FileInfo{}
	// log.Printf("file Imfo: %d", resp.Files[0].GetMtime())
	for _, f := range resp.Files {
		files = append(files, &types.FileInfo{
			Filename:     f.Filename,
			ModifiedTime: f.GetMtime(),
			Size:         f.Size,
			Checksum:     f.CrcChecksum,
			IsDeleted:    f.Deleted,
		})
	}

	return files, nil
}

func (c *Client) Store(ctx context.Context, filename string) error {
	// First acquire write access
	if err := c.AcquireWriteAccess(ctx, filename); err != nil {
		return fmt.Errorf("failed to acquire write access: %v", err)
	}

	// Start store stream
	stream, err := c.fsClient.Store(ctx)
	if err != nil {
		return fmt.Errorf("failed to start store stream: %v", err)
	}

	chunkSize := 1048 // 1KB
	fileStat, err := c.storage.StatFile(filename)
	if err != nil {
		return fmt.Errorf("failed to start store stream: %v", err)
	}

	err = c.storage.ReadFile(filename, chunkSize, func(chunk []byte) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := stream.Send(&pb.FileData{
				Filename:    filename,
				Content:     chunk,
				ClientId:    c.clientID,
				CrcChecksum: fileStat.Checksum,
			}); err != nil {
				return fmt.Errorf("failed to send chunk: %v", err)
			}
			return nil
		}
	})

	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Close the stream and get response
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to close stream: %v", err)
	}
	if !resp.Success {
		return fmt.Errorf("store operation failed")
	}

	return nil
}

func (c *Client) Fetch(ctx context.Context, filename string) error {
	boundedStream := channel.NewBoundedStream(20)

	errChan := make(chan error, 1)
	go func() {
		overwrite := true
		for {
			chunk, ok := boundedStream.Recv()
			if !ok {
				errChan <- nil
				return
			}

			err := c.storage.SaveFile(filename, chunk, overwrite)
			if err != nil {
				errChan <- fmt.Errorf("failed to save chunk: %v", err)
				return
			}
			overwrite = false // subsequent writes should append
		}
	}()

	// Start fetch stream
	stream, err := c.fsClient.Fetch(ctx, &pb.FetchRequest{
		Filename: filename,
		ClientId: c.clientID,
	})
	if err != nil {
		return fmt.Errorf("failed to start fetch stream: %v", err)
	}

	// Receive chunks from server and send to bounded stream
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			boundedStream.Close()
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive chunk: %v", err)
		}

		boundedStream.Send(chunk.Content)
	}

	// Wait for file saving to complete
	if err := <-errChan; err != nil {
		return err
	}

	return nil
}
