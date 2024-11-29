package server

import (
	"fmt"
	"sync"
	"time"
)

const (
	defaultLockTimeout = 5 * time.Minute
)

type LockManager interface {
	// Acquire attempts to acquire a lock for a file
	Acquire(filename, clientID string) error

	// Release releases a lock if held by the specified client
	Release(filename, clientID string) error

	// Check verifies if a file is locked by a different client
	Check(filename, clientID string) error

	// Close stops the lock manager and cleanup routines
	Close()
}

// LockInfo represents details about a file lock
type LockInfo struct {
	ClientID  string
	Acquired  time.Time
	ExpiresAt time.Time
}

// IsExpired checks if the lock has expired
func (l *LockInfo) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

type lockManager struct {
	locks     map[string]*LockInfo
	mu        sync.RWMutex
	closeOnce sync.Once
	done      chan struct{}
}

// NewLockManager creates a new instance of LockManager
func NewLockManager() LockManager {
	lm := &lockManager{
		locks: make(map[string]*LockInfo),
		done:  make(chan struct{}),
	}

	go lm.cleanupExpiredLocks()
	return lm
}

func (lm *lockManager) Acquire(filename, clientID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lock, exists := lm.locks[filename]; exists {
		// Allow re-entrant locks for the same client
		if lock.ClientID == clientID {
			lock.ExpiresAt = time.Now().Add(defaultLockTimeout)
			return nil
		}

		// Check if existing lock has expired
		if lock.IsExpired() {
			delete(lm.locks, filename)
		} else {
			return fmt.Errorf("file %s is locked by client %s until %s",
				filename, lock.ClientID, lock.ExpiresAt.Format(time.RFC3339))
		}
	}

	// Acquire new lock
	lm.locks[filename] = &LockInfo{
		ClientID:  clientID,
		Acquired:  time.Now(),
		ExpiresAt: time.Now().Add(defaultLockTimeout),
	}

	return nil
}

func (lm *lockManager) Release(filename, clientID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lock, exists := lm.locks[filename]
	if !exists {
		return nil
	}

	if lock.ClientID != clientID {
		return fmt.Errorf("cannot release lock owned by different client")
	}

	delete(lm.locks, filename)
	return nil
}

func (lm *lockManager) Check(filename, clientID string) error {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	lock, exists := lm.locks[filename]
	if !exists {
		return nil
	}

	if lock.IsExpired() {
		// Switch to write lock to delete expired lock
		lm.mu.RUnlock()
		lm.mu.Lock()
		defer lm.mu.Unlock()

		// Check again after acquiring write lock
		if lock, exists = lm.locks[filename]; exists && lock.IsExpired() {
			delete(lm.locks, filename)
		}
		return nil
	}

	if lock.ClientID != clientID {
		return fmt.Errorf("file %s is locked by client %s until %s",
			filename, lock.ClientID, lock.ExpiresAt.Format(time.RFC3339))
	}

	return nil
}

func (lm *lockManager) Close() {
	lm.closeOnce.Do(func() {
		close(lm.done)
	})
}

func (lm *lockManager) cleanupExpiredLocks() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-lm.done:
			return
		case <-ticker.C:
			lm.mu.Lock()
			for filename, lock := range lm.locks {
				if lock.IsExpired() {
					delete(lm.locks, filename)
				}
			}
			lm.mu.Unlock()
		}
	}
}
