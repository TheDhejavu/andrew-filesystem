package server

import (
	"sync"
	"time"

	"github.com/TheDhejavu/afs-protocol/internal/common/types"
)

type Tombstone struct {
	records  map[string]TombstoneRecord
	mu       sync.RWMutex
	done     chan struct{}
	stopOnce sync.Once
}

type TombstoneRecord struct {
	ttl          time.Time
	deleted_time time.Time
}

var (
	PurgeTTL      = 1 * time.Hour
	PurgeInterval = 5 * time.Minute
)

func NewTombstone() *Tombstone {
	t := &Tombstone{
		records: make(map[string]TombstoneRecord),
		done:    make(chan struct{}),
	}

	go t.startPeriodicPurge()
	return t
}

// Insert newly deleted data into tombstone to mark as deleted
func (t *Tombstone) Insert(filename string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.records[filename]; ok {
		return
	}
	t.records[filename] = TombstoneRecord{ttl: time.Now().Add(PurgeTTL), deleted_time: time.Now()}
}

// Remove from tombstone
func (t *Tombstone) Remove(filename string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.records[filename]; !ok {
		return
	}
	delete(t.records, filename)
}

func (t *Tombstone) Get(filename string) (TombstoneRecord, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ttl, ok := t.records[filename]
	return ttl, ok
}

// IsDeleted checks if a file is marked as deleted
func (t *Tombstone) IsDeleted(filename string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	_, ok := t.records[filename]
	return ok
}

func (t *Tombstone) MergeWithFiles(files []*types.FileInfo) []*types.FileInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// For each deleted file, add it to the files list
	for filename, record := range t.records {
		files = append(files, &types.FileInfo{
			Filename:     filename,
			IsDeleted:    true,
			ModifiedTime: record.deleted_time.Unix(),
		})
	}
	return files
}

// Stop gracefully stops the purge goroutine
func (t *Tombstone) Stop() {
	t.stopOnce.Do(func() {
		close(t.done)
	})
}

// Purge after a while to save memory
func (t *Tombstone) startPeriodicPurge() {
	ticker := time.NewTicker(PurgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.purgeExpired()
		case <-t.done:
			return
		}
	}
}

// purgeExpired removes expired entries
func (t *Tombstone) purgeExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for filename, record := range t.records {
		if now.After(record.ttl) {
			delete(t.records, filename)
		}
	}
}
