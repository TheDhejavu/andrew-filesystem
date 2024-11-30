package client

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type AFSWatcher struct {
	watcher *fsnotify.Watcher
	client  *Client
	ctx     context.Context
	cancel  context.CancelFunc
	coord   *Coordinator
}

func NewAFSWatcher(client *Client, coord *Coordinator) (*AFSWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &AFSWatcher{
		watcher: watcher,
		client:  client,
		ctx:     ctx,
		cancel:  cancel,
		coord:   coord,
	}, nil
}

func (w *AFSWatcher) Watch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	if err := w.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to add watch path: %v", err)
	}

	go w.handleEvents()

	log.Printf("Watching directory: %s\n", absPath)
	return nil
}

func (w *AFSWatcher) handleEvents() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			w.coord.mu.Lock()

			filename := filepath.Base(event.Name)
			ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)

			switch {
			case event.Has(fsnotify.Write):
				// On write, store the file to AFS
				log.Printf("Modified file: %s, syncing to AFS\n", filename)
				if err := w.client.Store(ctx, filename); err != nil {
					log.Printf("Failed to store file %s: %v\n", filename, err)
				}

			case event.Has(fsnotify.Create):
				log.Printf("Created file: %s, syncing to AFS\n", filename)
				if err := w.client.Store(ctx, filename); err != nil {
					log.Printf("Failed to store new file %s: %v\n", filename, err)
				}

			case event.Has(fsnotify.Remove):
				log.Printf("Removed file: %s, deleting from AFS\n", filename)
				if err := w.client.Delete(ctx, filename); err != nil {
					log.Printf("Failed to delete file %s: %v\n", filename, err)
				}

			case event.Has(fsnotify.Rename):
				// For rename, we treat it as a removal since we can't track the new name
				log.Printf("Renamed/Moved file: %s, handling as deletion in AFS\n", filename)
				if err := w.client.Delete(ctx, filename); err != nil {
					log.Printf("Failed to handle renamed file %s: %v\n", filename, err)
				}
			}
			w.coord.mu.Unlock()
			cancel()
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v\n", err)
		}
	}
}

func (w *AFSWatcher) Close() error {
	w.cancel()
	return w.watcher.Close()
}
