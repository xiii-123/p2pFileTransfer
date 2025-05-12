package file

import (
	"io"
	"os"
)

type FileSystemAdapter interface {
	Open(path string) (*io.ReadWriter, error)
}

type LocalFileSystemAdapter struct{}

func (LocalFileSystemAdapter) Open(path string) (*io.ReadWriter, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	readWriter := io.ReadWriter(file)
	return &readWriter, nil
}

func NewLocalFileSystemAdapter() FileSystemAdapter {
	return LocalFileSystemAdapter{}
}
