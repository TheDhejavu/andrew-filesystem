package server

import (
	"context"
	"fmt"
	"io"

	"github.com/TheDhejavu/afs-protocol/internal/common/channel"
	pb "github.com/TheDhejavu/afs-protocol/internal/proto/gen"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DFSHandler struct {
	pb.UnimplementedFileSystemServiceServer
	fileService FileService
	queue       chan AsyncRequest
}

var (
	QUEUE_SIZE = 1000
)

func NewDFSHandler(fileService FileService) *DFSHandler {
	handler := &DFSHandler{
		fileService: fileService,
		queue:       make(chan AsyncRequest, QUEUE_SIZE),
	}

	// Start the queue processor
	go handler.processQueue()

	return handler
}

func (h *DFSHandler) AcquireWriteAccess(ctx context.Context, req *pb.WriteAccessRequest) (*pb.WriteAccessResponse, error) {
	err := h.fileService.AcquireWriteLock(ctx, req.Filename, req.ClientId)
	if err != nil {
		return &pb.WriteAccessResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.WriteAccessResponse{
		Success: true,
	}, nil
}

func (h *DFSHandler) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {

	err := h.fileService.Delete(ctx, req.Filename, req.ClientId)
	if err != nil {
		return &pb.DeleteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if err := h.fileService.ReleaseLock(ctx, req.Filename, req.ClientId); err != nil {
		log.Error().Msgf("Unable to release lock: %v", err)
	}

	return &pb.DeleteResponse{
		Success: false,
	}, nil
}

func (h *DFSHandler) GetFileStat(ctx context.Context, req *pb.GetFileStatRequest) (*pb.File, error) {
	file, err := h.fileService.GetFileStat(ctx, req.Filename)
	if err != nil {
		return nil, fmt.Errorf("unable to query file state: %w", err)
	}

	return &pb.File{
		Filename:    file.Filename,
		Size:        file.Size,
		Mtime:       file.ModifiedTime,
		CrcChecksum: file.Checksum,
	}, nil
}

func (h *DFSHandler) Store(stream pb.FileSystemService_StoreServer) error {
	boundedStream := channel.NewBoundedStream(20)

	// Get first chunk to initialize
	chunk, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to receive initial chunk: %v", err)
	}

	errChan := make(chan error, 1)
	go func() {
		err := h.fileService.Store(stream.Context(), chunk.Filename, chunk.ClientId, boundedStream)
		errChan <- err
	}()

	if chunk.Filename == "" || chunk.ClientId == "" {
		return status.Errorf(codes.InvalidArgument, "filename and client_id are required")
	}

	defer func() {
		if err := h.fileService.ReleaseLock(stream.Context(), chunk.Filename, chunk.ClientId); err != nil {
			log.Error().Msgf("Unable to release lock: %v", err)
		}
	}()

	// Send first chunk
	boundedStream.Send(chunk.Content)

	// Stream remaining chunks
	for {
		select {
		case err := <-errChan:
			if err != nil {
				return status.Errorf(codes.Internal, "store operation failed: %v", err)
			}
		default:
		}

		chunk, err := stream.Recv()
		if err == io.EOF {
			boundedStream.Close()

			// Wait for store operation to complete
			if err := <-errChan; err != nil {
				return status.Errorf(codes.Internal, "store operation failed: %v", err)
			}

			return stream.SendAndClose(&pb.StoreResponse{
				Success: true,
			})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive chunk: %v", err)
		}

		boundedStream.Send(chunk.Content)
	}
}

func (h *DFSHandler) Fetch(req *pb.FetchRequest, stream pb.FileSystemService_FetchServer) error {
	boundedStream := channel.NewBoundedStream(20)

	errChan := make(chan error, 1)
	go func() {
		// Stream data chunks to client
		for {
			chunk, ok := boundedStream.Recv()
			if !ok {
				errChan <- nil
				return
			}

			err := stream.Send(&pb.FileData{
				Filename: req.Filename,
				ClientId: req.ClientId,
				Content:  chunk,
			})
			if err != nil {
				errChan <- status.Errorf(codes.Internal, err.Error())
				break
			}
		}
		errChan <- nil
	}()

	err := h.fileService.Fetch(stream.Context(), req.Filename, boundedStream)
	if err != nil {
		return err
	}

	return <-errChan
}

type AsyncRequest struct {
	responseChan chan *pb.RequestFilesAsyncResponse
	clientID     string
}

func (h *DFSHandler) RequestFilesAsync(ctx context.Context, req *pb.RequestFilesAsyncRequest) (*pb.RequestFilesAsyncResponse, error) {
	// Create a response channel for this client
	responseChan := make(chan *pb.RequestFilesAsyncResponse, 1)
	clientID := req.ClientId

	// Send the request to the queue for processing
	h.queue <- AsyncRequest{clientID: clientID, responseChan: responseChan}

	// Wait for a response or context cancellation
	select {
	case <-ctx.Done():
		return nil, status.Errorf(codes.Canceled, "Request cancelled")
	case response := <-responseChan:
		return response, nil
	}
}

// Process queued requests and send responses
func (h *DFSHandler) processQueue() {
	for req := range h.queue {
		currentFiles, err := h.fileService.ListFiles(context.Background())
		if err != nil {
			continue
		}

		files := []*pb.File{}
		for _, f := range currentFiles {
			files = append(files, &pb.File{
				CrcChecksum: f.Checksum,
				Mtime:       f.ModifiedTime,
				Filename:    f.Filename,
				Deleted:     false,
				Size:        f.Size,
			})
		}

		response := &pb.RequestFilesAsyncResponse{
			Files: files,
		}

		// Send the response through the channel
		req.responseChan <- response
	}
}
