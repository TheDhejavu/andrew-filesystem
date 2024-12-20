package client

import (
	"context"
	"errors"
	"os"

	"time"

	"github.com/TheDhejavu/afs-protocol/internal/common/types"
	"github.com/rs/zerolog/log"
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
		log.Error().Err(err).Msg("Sync failed")
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
		if errors.Is(err, os.ErrNotExist) {
			if remoteFile.IsDeleted {
				// Both agree file doesn't exist - nothing to do
				return
			}
			// File exists remotely but not locally - fetch it
			w.handleMissingFile(ctx, remoteFile)
			return
		}
		log.Error().Err(err).Str("file", remoteFile.Filename).Msg("Error checking local file")
		return
	}
	if localFile == nil {
		return
	}

	// Delete local file when the remote deletion is more recent (has a newer timestamp) than our local version,
	//  ensuring we respect deletions that happened after our last sync.
	if remoteFile.IsDeleted && remoteFile.ModifiedTime > localFile.ModifiedTime {
		if err := w.client.storage.DeleteFile(remoteFile.Filename); err != nil {
			log.Error().Err(err).Str("file", remoteFile.Filename).Msg("Failed to delete local file")
		}
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
