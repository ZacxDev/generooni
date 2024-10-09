package executor

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/ZacxDev/generooni/fs"
)

type LockFileEntry struct {
	CachedFiles map[string]string
}

type LockFileManager interface {
	LoadLockFile() error
	SaveFreshLockFile() error
	GetCachedEntry(string) (LockFileEntry, bool)
	AddFreshEntry(string, LockFileEntry)

	// Getters
	LockFile() map[string]LockFileEntry
	FreshLockFile() map[string]LockFileEntry
	FS() fs.FileSystem

	// Setters
	SetLockFile(map[string]LockFileEntry)
	SetFreshLockFile(map[string]LockFileEntry)
	SetFS(fs.FileSystem)
}

type lockFileManager struct {
	lockFile      map[string]LockFileEntry
	freshLockFile map[string]LockFileEntry
	fs            fs.FileSystem
	mu            sync.Mutex
}

func NewLockFileManager(fs fs.FileSystem) LockFileManager {
	return &lockFileManager{
		lockFile:      make(map[string]LockFileEntry),
		freshLockFile: make(map[string]LockFileEntry),
		fs:            fs,
	}
}

func (lm *lockFileManager) LoadLockFile() error {
	data, err := lm.fs.ReadFile("generooni.lock")
	if err != nil {
		if os.IsNotExist(err) {
			return nil // It's okay if the lock file doesn't exist yet
		}
		return err
	}

	return json.Unmarshal(data, &lm.lockFile)
}

func (lm *lockFileManager) SaveFreshLockFile() error {
	data, err := json.MarshalIndent(lm.freshLockFile, "", "  ")
	if err != nil {
		return err
	}

	return lm.fs.WriteFile("generooni.lock", data, 0644)
}

func (lm *lockFileManager) GetCachedEntry(key string) (LockFileEntry, bool) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	entry, exists := lm.lockFile[key]
	return entry, exists
}

func (lm *lockFileManager) AddFreshEntry(key string, entry LockFileEntry) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if entry.CachedFiles == nil {
		entry.CachedFiles = make(map[string]string)
	}
	lm.freshLockFile[key] = entry
}

func (lfm *lockFileManager) LockFile() map[string]LockFileEntry {
	lfm.mu.Lock()
	defer lfm.mu.Unlock()
	return lfm.lockFile
}

func (lfm *lockFileManager) FreshLockFile() map[string]LockFileEntry {
	lfm.mu.Lock()
	defer lfm.mu.Unlock()
	return lfm.freshLockFile
}

func (lfm *lockFileManager) FS() fs.FileSystem {
	lfm.mu.Lock()
	defer lfm.mu.Unlock()
	return lfm.fs
}

// Setters

func (lfm *lockFileManager) SetLockFile(lf map[string]LockFileEntry) {
	lfm.mu.Lock()
	defer lfm.mu.Unlock()
	lfm.lockFile = lf
}

func (lfm *lockFileManager) SetFreshLockFile(flf map[string]LockFileEntry) {
	lfm.mu.Lock()
	defer lfm.mu.Unlock()
	lfm.freshLockFile = flf
}

func (lfm *lockFileManager) SetFS(filesystem fs.FileSystem) {
	lfm.mu.Lock()
	defer lfm.mu.Unlock()
	lfm.fs = filesystem
}
