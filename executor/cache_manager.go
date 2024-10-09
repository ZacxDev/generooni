package executor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	iofs "io/fs"

	"github.com/ZacxDev/generooni/fs"
	"github.com/ZacxDev/generooni/target"
	"github.com/pkg/errors"
)

type CacheManager interface {
	EnsureCacheDir() error
	ApplyCachedFileChanges(LockFileEntry) error
	CollectAndStoreFileChanges(*target.FilesystemTarget) (*LockFileEntry, error)

	// Getters
	FS() fs.FileSystem
	CacheDir() string

	// Setters
	SetFS(fs.FileSystem)
	SetCacheDir(string)

	// These methods are now part of the interface
	restoreFile(cachedPath, originalPath string) error
	cacheFile(originalPath string) (string, error)
}

type cacheManager struct {
	fs       fs.FileSystem
	cacheDir string
}

func NewCacheManager(fs fs.FileSystem, cacheDir string) CacheManager {
	return &cacheManager{
		fs:       fs,
		cacheDir: cacheDir,
	}
}

func (cm *cacheManager) ApplyCachedFileChanges(entry LockFileEntry) error {
	if err := cm.verifyCacheIntegrity(entry); err != nil {
		return fmt.Errorf("cache integrity check failed: %w", err)
	}

	for originalPath, cachedPath := range entry.CachedFiles {
		if err := cm.restoreFile(cachedPath, originalPath); err != nil {
			return fmt.Errorf("error restoring file %s: %w", originalPath, err)
		}
	}

	return nil
}

func (cm *cacheManager) CollectAndStoreFileChanges(target *target.FilesystemTarget) (*LockFileEntry, error) {
	entry := LockFileEntry{
		CachedFiles: make(map[string]string),
	}

	for _, pattern := range target.Outputs {
		if err := cm.processGlobPattern(pattern, &entry); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &entry, nil
}

func (cm *cacheManager) processGlobPattern(pattern string, entry *LockFileEntry) error {
	matches, err := cm.fs.DoublestarGlob(pattern)
	if err != nil {
		return fmt.Errorf("error expanding glob pattern %s: %w", pattern, err)
	}

	for _, match := range matches {
		if err := cm.processMatch(match, entry); err != nil {
			return err
		}
	}

	return nil
}

func (cm *cacheManager) processMatch(match string, entry *LockFileEntry) error {
	return cm.fs.WalkDir(match, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			cachedPath, err := cm.cacheFile(path)
			if err != nil {
				return fmt.Errorf("error caching file %s: %w", path, err)
			}
			entry.CachedFiles[path] = cachedPath
		}
		return nil
	})
}

func (cm *cacheManager) verifyCacheIntegrity(entry LockFileEntry) error {
	for _, cachedPath := range entry.CachedFiles {
		if _, err := cm.fs.Stat(cachedPath); os.IsNotExist(err) {
			return fmt.Errorf("cached file %s is missing", cachedPath)
		}
	}
	return nil
}

func (cm *cacheManager) restoreFile(cachedPath, originalPath string) error {
	content, err := cm.fs.ReadFile(cachedPath)
	if err != nil {
		return fmt.Errorf("error reading cached file %s: %w", cachedPath, err)
	}

	originalInfo, err := cm.fs.Stat(originalPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error stating original file %s: %w", originalPath, err)
	}

	fileMode := cm.determineFileMode(originalInfo, err)

	if err := cm.fs.MkdirAll(filepath.Dir(originalPath), 0755); err != nil {
		return fmt.Errorf("error creating directory for %s: %w", originalPath, err)
	}

	if err := cm.writeFile(originalPath, content, fileMode); err != nil {
		return fmt.Errorf("error writing restored file %s: %w", originalPath, err)
	}

	return nil
}

func (cm *cacheManager) determineFileMode(originalInfo os.FileInfo, err error) os.FileMode {
	if err == nil {
		return originalInfo.Mode()
	}
	return 0644 // Default mode if file doesn't exist
}

func (cm *cacheManager) writeFile(path string, content []byte, mode os.FileMode) error {
	tempFile := path + ".tmp"
	if err := cm.fs.WriteFile(tempFile, content, mode); err != nil {
		return err
	}
	return cm.fs.Rename(tempFile, path)
}

func (cm *cacheManager) cacheFile(originalPath string) (string, error) {
	content, err := cm.fs.ReadFile(originalPath)
	if err != nil {
		return "", fmt.Errorf("error reading file %s: %w", originalPath, err)
	}

	hash := sha256.Sum256(content)
	hashString := hex.EncodeToString(hash[:])

	cachedPath := filepath.Join(cm.cacheDir, hashString)
	if err := cm.fs.MkdirAll(filepath.Dir(cachedPath), 0755); err != nil {
		return "", fmt.Errorf("error creating cache directory: %w", err)
	}

	if err := cm.fs.WriteFile(cachedPath, content, 0644); err != nil {
		return "", fmt.Errorf("error writing cached file: %w", err)
	}

	return cachedPath, nil
}

func (cm *cacheManager) EnsureCacheDir() error {
	return cm.fs.MkdirAll(cm.cacheDir, 0755)
}

func (cm *cacheManager) FS() fs.FileSystem {
	return cm.fs
}

func (cm *cacheManager) CacheDir() string {
	return cm.cacheDir
}

func (cm *cacheManager) SetFS(filesystem fs.FileSystem) {
	cm.fs = filesystem
}

func (cm *cacheManager) SetCacheDir(dir string) {
	cm.cacheDir = dir
}
