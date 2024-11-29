// internal/server/service/service.go
package server

import (
	"context"
	"fmt"

	"github.com/TheDhejavu/afs-protocol/internal/common/types"
)

type FileService interface {
	AcquireWriteLock(ctx context.Context, filename, clientID string) error
	ReleaseLock(ctx context.Context, filename, clientID string) error
	ListFiles(ctx context.Context) ([]*types.FileInfo, error)

	// Operations
	Store(ctx context.Context, filename string, clientID string, stream *BoundedStream) error
	Delete(ctx context.Context, filename string, clientID string) error
	Fetch(ctx context.Context, filename string, stream *BoundedStream) error
	GetFileStat(ctx context.Context, filename string) (*types.FileInfo, error)
}

type fileService struct {
	storage     Storage
	lockManager LockManager
}

func NewFileService(storage Storage) FileService {
	return &fileService{
		lockManager: NewLockManager(),
		storage:     storage,
	}
}

func (s *fileService) AcquireWriteLock(ctx context.Context, filename, clientID string) error {
	return s.lockManager.Acquire(filename, clientID)
}

func (s *fileService) ReleaseLock(ctx context.Context, filename, clientID string) error {
	return s.lockManager.Release(filename, clientID)
}

func (s *fileService) ListFiles(ctx context.Context) ([]*types.FileInfo, error) {
	return s.storage.ListFiles()
}

func (s *fileService) Store(ctx context.Context, filename string, clientID string, stream *BoundedStream) error {
	if err := s.lockManager.Check(filename, clientID); err != nil {
		return err
	}

	overwrite := true
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Receive the next chunk of data
			chunk, ok := stream.Recv()
			if !ok {
				return nil
			}

			err := s.storage.SaveFile(filename, chunk, overwrite)
			if err != nil {
				return fmt.Errorf("failed to save file: %v", err)
			}
			overwrite = false
		}
	}
}

func (s *fileService) Fetch(ctx context.Context, filename string, stream *BoundedStream) error {
	defer stream.Close()

	chunkSize := 1000
	err := s.storage.ReadFile(filename, chunkSize, func(chunk []byte) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			stream.Send(chunk)
			return nil
		}
	})

	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}

func (s *fileService) Delete(ctx context.Context, filename string, clientID string) error {
	if err := s.lockManager.Check(filename, clientID); err != nil {
		return err
	}

	err := s.storage.DeleteFile(filename)
	if err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}

	return nil
}

func (s *fileService) GetFileStat(ctx context.Context, filename string) (*types.FileInfo, error) {
	if ok, err := s.storage.FileExists(filename); err != nil && !ok {
		return nil, fmt.Errorf("unable to get file stat: %v", err)
	}

	return s.storage.StatFile(filename)
}
