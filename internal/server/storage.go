package server

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"

	"github.com/TheDhejavu/afs-protocol/internal/common/types"
)

type Storage interface {
	FileExists(filename string) (bool, error)
	SaveFile(filename string, content []byte, overwrite bool) error
	DeleteFile(filename string) error
	ReadFile(filename string, chunkSize int, callback ChunkCallback) error
	StatFile(filename string) (*types.FileInfo, error)
	ListFiles() ([]*types.FileInfo, error)
}

type ChunkCallback func(chunk []byte) error

type diskStorage struct {
	mountPath string
}

func NewDiskStorage(mountPath string) (Storage, error) {
	if err := ensureStorageDir(mountPath); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %v", err)
	}

	return &diskStorage{
		mountPath: mountPath,
	}, nil
}

func (s *diskStorage) FileExists(filename string) (bool, error) {
	_, err := os.Stat(filepath.Join(s.mountPath, filename))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *diskStorage) StatFile(filename string) (*types.FileInfo, error) {
	file, err := os.Stat(filepath.Join(s.mountPath, filename))
	if err == nil {
		return nil, nil
	}
	if os.IsNotExist(err) {
		return nil, nil
	}

	checksum, err := getChecksum(filepath.Join(s.mountPath, filename))
	if err != nil {
		return nil, err
	}
	return &types.FileInfo{
		Filename:     filename,
		Size:         file.Size(),
		ModifiedTime: file.ModTime().Unix(),
		Checksum:     checksum,
	}, err
}

// SaveFile writes received byte stream incrementally to a file and it overwrites
// the file if it's new, and appends content if it's already there.
func (s *diskStorage) SaveFile(filename string, content []byte, overwrite bool) error {
	filePath := filepath.Join(s.mountPath, filename)

	flags := os.O_RDWR | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}

	file, err := os.OpenFile(filePath, flags, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filename, err)
	}
	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %v", filename, err)
	}

	return nil
}

func (s *diskStorage) DeleteFile(filename string) error {
	filePath := filepath.Join(s.mountPath, filename)
	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to delete file %s: %v", filename, err)
	}
	return nil
}

// ReadFile reads the file in chunks and calls the provided callback with each chunk.
func (s *diskStorage) ReadFile(filename string, chunkSize int, callback ChunkCallback) error {
	file, err := os.Open(filepath.Join(s.mountPath, filename))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	buffer := make([]byte, chunkSize)

	for {
		// Read the file in chunks
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// If we reached EOF, break out of the loop
		if n == 0 {
			break
		}

		if err := callback(buffer[:n]); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}

	return nil
}
func ensureStorageDir(dir string) error {
	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}
	return nil
}

// ListFiles returns a list of files in the storage directory with their checksums
func (s *diskStorage) ListFiles() ([]*types.FileInfo, error) {
	files := []*types.FileInfo{}

	// Read all files in the directory
	err := filepath.Walk(s.mountPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, only process files
		if !info.IsDir() {
			checksum, err := getChecksum(path)
			if err != nil {
				return err
			}

			files = append(files, &types.FileInfo{
				Filename: info.Name(),
				Checksum: checksum,
			})
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %v", err)
	}

	return files, nil
}

// CRC3 calculates CRC-3 checksum for a file crc32 package
func getChecksum(filePath string) (uint32, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	// Initialize the CRC calculation state (start with a zero value)
	var crcValue uint32 = 0

	// Read 1KB of file in chunks
	buf := make([]byte, 1024)
	for {
		n, err := file.Read(buf)
		if err != nil && err.Error() != "EOF" {
			return 0, fmt.Errorf("failed to read file %s: %v", filePath, err)
		}
		if n == 0 {
			break
		}

		for i := 0; i < n; i++ {
			// Update the CRC value with each byte read
			crcValue = crc32.Update(crcValue, crc32.IEEETable, []byte{buf[i]})
		}
	}

	// Apply CRC-3 polynomial by masking out all but the 3 least significant bits
	crcValue &= 0x7

	return crcValue, nil
}
