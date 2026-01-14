// Package file 提供文件系统抽象层
//
// 功能:
//   - 文件打开: 统一的文件打开接口
//   - 抽象层: 隔离具体文件系统实现
//   - 可扩展: 支持自定义文件系统适配器
//
// 主要组件:
//   - FileSystemAdapter: 文件系统适配器接口
//   - LocalFileSystemAdapter: 本地文件系统实现
//
// 使用示例:
//
//	adapter := NewLocalFileSystemAdapter()
//	file, err := adapter.Open("/path/to/file")
//	if err != nil {
//	    return err
//	}
//	defer (*file).Close()
//
// 注意事项:
//   - 当前仅实现了本地文件系统
//   - 可扩展支持云存储、分布式文件系统等
//   - 返回 *io.ReadWriter 以支持读写操作
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
