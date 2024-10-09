package executor

import (
	"strings"
	"testing"
	"testing/quick"

	"github.com/ZacxDev/generooni/fs/mock"
	"github.com/ZacxDev/generooni/target"
)

func TestNewCacheManager(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	if cm.FS() != fs {
		t.Error("FileSystem not set correctly")
	}

	if cm.CacheDir() != ".cache" {
		t.Error("Cache directory not set correctly")
	}
}

func TestApplyCachedFileChanges(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	// Setup mock cached files
	fs.WriteFile(".cache/file1_hash", []byte("content1"), 0644)
	fs.WriteFile(".cache/file2_hash", []byte("content2"), 0644)

	entry := LockFileEntry{
		CachedFiles: map[string]string{
			"file1.txt": ".cache/file1_hash",
			"file2.txt": ".cache/file2_hash",
		},
	}

	err := cm.ApplyCachedFileChanges(entry)
	if err != nil {
		t.Fatalf("ApplyCachedFileChanges failed: %v", err)
	}

	// Check if files were restored correctly
	content1, _ := fs.ReadFile("file1.txt")
	content2, _ := fs.ReadFile("file2.txt")

	if string(content1) != "content1" || string(content2) != "content2" {
		t.Error("Files not restored correctly")
	}
}

func TestCollectAndStoreFileChanges(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	// Setup mock files
	fs.WriteFile("file1.txt", []byte("content1"), 0644)
	fs.WriteFile("file2.txt", []byte("content2"), 0644)
	fs.WriteFile("subdir/file3.txt", []byte("content3"), 0644)
	fs.WriteFile("file4.dat", []byte("content4"), 0644)

	target := &target.FilesystemTarget{
		Outputs: []string{"**/*.txt"},
	}

	entry, err := cm.CollectAndStoreFileChanges(target)
	if err != nil {
		t.Fatalf("CollectAndStoreFileChanges failed: %v", err)
	}

	if len(entry.CachedFiles) != 3 {
		t.Errorf("Expected 3 cached files, got %d", len(entry.CachedFiles))
	}

	// Check if files were cached correctly
	for originalPath, cachedPath := range entry.CachedFiles {
		if !strings.HasSuffix(originalPath, ".txt") {
			t.Errorf("Unexpected file cached: %s", originalPath)
		}
		if _, err := fs.Stat(cachedPath); err != nil {
			t.Errorf("Cached file %s not found", cachedPath)
		}
	}
}

func TestRestoreFile(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	// Setup mock cached file
	fs.WriteFile(".cache/file_hash", []byte("content"), 0644)

	err := cm.restoreFile(".cache/file_hash", "restored.txt")
	if err != nil {
		t.Fatalf("restoreFile failed: %v", err)
	}

	content, _ := fs.ReadFile("restored.txt")
	if string(content) != "content" {
		t.Error("File not restored correctly")
	}
}

func TestCacheFile(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	// Setup mock original file
	fs.WriteFile("original.txt", []byte("content"), 0644)

	cachedPath, err := cm.cacheFile("original.txt")
	if err != nil {
		t.Fatalf("cacheFile failed: %v", err)
	}

	if _, err := fs.Stat(cachedPath); err != nil {
		t.Errorf("Cached file %s not found", cachedPath)
	}

	cachedContent, _ := fs.ReadFile(cachedPath)
	if string(cachedContent) != "content" {
		t.Error("File not cached correctly")
	}
}

func TestEnsureCacheDir(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	err := cm.EnsureCacheDir()
	if err != nil {
		t.Fatalf("EnsureCacheDir failed: %v", err)
	}

	// Since MockFileSystem.MkdirAll always returns nil, we can't check if it was called.
	// In a real scenario, we could use a mock that tracks method calls.
}

// Property-based test
func TestCacheFileProperty(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	f := func(content string) bool {
		originalPath := "test.txt"
		fs.WriteFile(originalPath, []byte(content), 0644)

		cachedPath, err := cm.cacheFile(originalPath)
		if err != nil {
			return false
		}

		cachedContent, err := fs.ReadFile(cachedPath)
		if err != nil {
			return false
		}

		return string(cachedContent) == content
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

// Test error cases
func TestApplyCachedFileChangesErrors(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	entry := LockFileEntry{
		CachedFiles: map[string]string{
			"file1.txt": ".cache/nonexistent",
		},
	}

	err := cm.ApplyCachedFileChanges(entry)
	if err == nil {
		t.Error("Expected error for missing cached file, got nil")
	}
}

func TestRestoreFileErrors(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	err := cm.restoreFile(".cache/nonexistent", "restored.txt")
	if err == nil {
		t.Error("Expected error for nonexistent cached file, got nil")
	}
}

func TestCacheFileErrors(t *testing.T) {
	fs := mock.NewMockFileSystem()
	cm := NewCacheManager(fs, ".cache")

	_, err := cm.cacheFile("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for nonexistent original file, got nil")
	}
}
