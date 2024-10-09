package mock

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"

	iofs "io/fs"

	"github.com/ZacxDev/generooni/fs"
	"github.com/bmatcuk/doublestar/v4"
)

type MockFile struct {
	*bytes.Buffer
	ReadOnly bool
}

type mockDirEntry struct {
	name  string
	isDir bool
	typ   iofs.FileMode
	info  iofs.FileInfo
}

func (m *mockDirEntry) Name() string                 { return m.name }
func (m *mockDirEntry) IsDir() bool                  { return m.isDir }
func (m *mockDirEntry) Type() iofs.FileMode          { return m.typ }
func (m *mockDirEntry) Info() (iofs.FileInfo, error) { return m.info, nil }

type mockFileInfo struct {
	name string
	mode os.FileMode
	size int64
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.mode.IsDir() }
func (m *mockFileInfo) Sys() interface{}   { return nil }

func (m *MockFile) Close() error {
	return nil
}

func (m *MockFile) Write(p []byte) (n int, err error) {
	if m.ReadOnly {
		return 0, os.ErrPermission
	}
	return m.Buffer.Write(p)
}

// MockFileSystem implements the FileSystem interface for testing
type MockFileSystem struct {
	Files    map[string]*MockFile
	fileMode map[string]os.FileMode
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		Files:    make(map[string]*MockFile),
		fileMode: make(map[string]os.FileMode),
	}
}

func (m *MockFileSystem) ReadFile(filename string) ([]byte, error) {
	if file, ok := m.Files[filename]; ok {
		if file.ReadOnly {
			return nil, os.ErrPermission
		}
		return file.Bytes(), nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if file, ok := m.Files[filename]; ok && file.ReadOnly {
		return os.ErrPermission
	}
	m.Files[filename] = &MockFile{Buffer: bytes.NewBuffer(data)}
	m.fileMode[filename] = perm

	return nil
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if _, ok := m.Files[name]; ok {
		return nil, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) Open(name string) (fs.File, error) {
	if file, ok := m.Files[name]; ok {
		return file, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) Create(name string) (fs.File, error) {
	file := &MockFile{Buffer: bytes.NewBuffer(nil)}
	m.Files[name] = file
	return file, nil
}

func (m *MockFileSystem) Rename(oldpath, newpath string) error {
	if data, ok := m.Files[oldpath]; ok {
		m.Files[newpath] = data
		m.fileMode[newpath] = m.fileMode[oldpath]
		delete(m.Files, oldpath)
		delete(m.fileMode, oldpath)
		return nil
	}
	return os.ErrNotExist
}

func (m *MockFileSystem) DoublestarGlob(pattern string) ([]string, error) {
	var matches []string
	for filename := range m.Files {
		matched, err := doublestar.Match(pattern, filename)
		if err != nil {
			return nil, err
		}
		if matched {
			matches = append(matches, filename)
		}
	}
	return matches, nil
}

func (m *MockFileSystem) WalkDir(root string, fn iofs.WalkDirFunc) error {
	for path := range m.Files {
		if strings.HasPrefix(path, root) {
			info := &mockDirEntry{
				name:  filepath.Base(path),
				isDir: false,
				typ:   os.FileMode(0),
				info: &mockFileInfo{
					name: filepath.Base(path),
					mode: m.fileMode[path],
					size: 0,
				},
			}
			if err := fn(path, info, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
