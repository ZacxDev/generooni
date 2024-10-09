package fs

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

type File interface {
	io.ReadCloser
	io.WriteCloser
}

// FileSystem interface for dependency injection and improved testability
type FileSystem interface {
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	Open(name string) (File, error)
	Create(name string) (File, error)
	Rename(oldpath, newpath string) error
	DoublestarGlob(pattern string) ([]string, error)
	WalkDir(root string, walkFn fs.WalkDirFunc) error
}

// RealFileSystem implements FileSystem interface using actual OS calls
type RealFileSystem struct{}

func (RealFileSystem) ReadFile(filename string) ([]byte, error) { return os.ReadFile(filename) }
func (RealFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}
func (RealFileSystem) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (RealFileSystem) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (RealFileSystem) Open(name string) (File, error)               { return os.Open(name) }
func (RealFileSystem) Create(name string) (File, error)             { return os.Create(name) }
func (RealFileSystem) Rename(oldpath, newpath string) error         { return os.Rename(oldpath, newpath) }
func (RealFileSystem) DoublestarGlob(pattern string) ([]string, error) {
	return doublestar.FilepathGlob(pattern)
}
func (RealFileSystem) WalkDir(root string, walkFn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, walkFn)
}
