package types

type FileInfo struct {
	Filename     string
	Checksum     uint32
	Size         int64
	ModifiedTime int64
	IsDeleted    bool
}
