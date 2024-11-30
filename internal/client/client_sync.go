package client

import (
	"context"
	"log"
	"time"

	"github.com/TheDhejavu/afs-protocol/internal/common/types"
)

type FileSync struct {
	client *Client
	coord  *Coordinator
}

func NewFileSync(client *Client, coord *Coordinator) *FileSync {
	return &FileSync{client: client, coord: coord}
}

func (w *FileSync) Start(ctx context.Context) {
	w.requestAndSync(ctx)
}

func (w *FileSync) requestAndSync(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
		w.processRemoteFiles(ctx)

		time.Sleep(1000)
		w.requestAndSync(ctx)
	}
}

func (w *FileSync) processRemoteFiles(ctx context.Context) {
	remoteFiles, err := w.client.RequestFilesAsync(ctx)
	if err != nil {
		log.Printf("Sync failed: %v", err)
		return
	}

	for _, remoteFile := range remoteFiles {
		w.syncFile(ctx, remoteFile)
	}
}

func (w *FileSync) syncFile(ctx context.Context, remoteFile *types.FileInfo) {
	w.coord.mu.Lock()
	defer w.coord.mu.Unlock()

	localFile, err := w.client.storage.StatFile(remoteFile.Filename)
	if err != nil {
		w.handleMissingFile(ctx, remoteFile)
		return
	}
	if localFile == nil {
		return
	}

	if localFile.Checksum != remoteFile.Checksum {
		w.handleChecksumMismatch(ctx, localFile, remoteFile)
	}
}

func (w *FileSync) handleMissingFile(ctx context.Context, remoteFile *types.FileInfo) {
	log.Printf("File %s not found locally, fetching", remoteFile.Filename)
	if err := w.client.Fetch(ctx, remoteFile.Filename); err != nil {
		log.Printf("Failed to fetch file %s: %v", remoteFile.Filename, err)
	}
}

func (w *FileSync) handleChecksumMismatch(ctx context.Context, localFile, remoteFile *types.FileInfo) {
	log.Printf("Local file: %d", localFile.ModifiedTime)
	log.Printf("Remote file: %d", remoteFile.ModifiedTime)
	if localFile.ModifiedTime > remoteFile.ModifiedTime {
		w.uploadFile(ctx, remoteFile.Filename)
	} else {
		w.downloadFile(ctx, remoteFile.Filename)
	}
}

func (w *FileSync) uploadFile(ctx context.Context, filename string) {
	log.Printf("Local file %s is newer, uploading to server", filename)
	if err := w.client.Store(ctx, filename); err != nil {
		log.Printf("Failed to upload file %s: %v", filename, err)
	}
}

func (w *FileSync) downloadFile(ctx context.Context, filename string) {
	log.Printf("Remote file %s is newer or changed, downloading", filename)
	if err := w.client.Fetch(ctx, filename); err != nil {
		log.Printf("Failed to download file %s: %v", filename, err)
	}
}
