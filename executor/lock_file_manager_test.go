package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/ZacxDev/generooni/fs/mock"
)

// TestNewLockFileManager tests the creation of a new LockFileManager
func TestNewLockFileManager(t *testing.T) {
	fs := mock.NewMockFileSystem()
	lm := NewLockFileManager(fs)

	if lm == nil {
		t.Fatal("NewLockFileManager returned nil")
	}

	if lm.FS() != fs {
		t.Error("FileSystem not set correctly")
	}

	if len(lm.LockFile()) != 0 {
		t.Error("lockFile should be empty on creation")
	}

	if len(lm.FreshLockFile()) != 0 {
		t.Error("freshLockFile should be empty on creation")
	}
}

// TestLoadLockFile tests the LoadLockFile method
func TestLoadLockFile(t *testing.T) {
	fs := mock.NewMockFileSystem()
	lm := NewLockFileManager(fs)

	// Test loading non-existent lock file
	err := lm.LoadLockFile()
	if err != nil {
		t.Errorf("LoadLockFile should not return an error for non-existent file: %v", err)
	}

	// Test loading existing lock file
	testData := map[string]LockFileEntry{
		"key1": {CachedFiles: map[string]string{"file1": "cache1"}},
	}
	testDataJSON, _ := json.Marshal(testData)
	fs.WriteFile("generooni.lock", testDataJSON, 0644)

	err = lm.LoadLockFile()
	if err != nil {
		t.Errorf("LoadLockFile failed: %v", err)
	}

	if !reflect.DeepEqual(lm.LockFile(), testData) {
		t.Error("LoadLockFile did not load data correctly")
	}

	// Test loading invalid JSON
	fs.WriteFile("generooni.lock", []byte("invalid json"), 0644)
	err = lm.LoadLockFile()
	if err == nil {
		t.Error("LoadLockFile should return an error for invalid JSON")
	}
}

// TestSaveFreshLockFile tests the SaveFreshLockFile method
func TestSaveFreshLockFile(t *testing.T) {
	fs := mock.NewMockFileSystem()
	lm := NewLockFileManager(fs)

	testData := map[string]LockFileEntry{
		"key1": {CachedFiles: map[string]string{"file1": "cache1"}},
	}
	lm.SetFreshLockFile(testData)

	err := lm.SaveFreshLockFile()
	if err != nil {
		t.Errorf("SaveFreshLockFile failed: %v", err)
	}

	savedData, _ := fs.ReadFile("generooni.lock")
	var savedLockFile map[string]LockFileEntry
	err = json.Unmarshal(savedData, &savedLockFile)
	if err != nil {
		t.Errorf("Failed to unmarshal saved lock file: %v", err)
	}

	if !reflect.DeepEqual(savedLockFile, testData) {
		t.Errorf("SaveFreshLockFile did not save data correctly. Expected: %v, Got: %v", testData, savedLockFile)
	}
}

// TestGetCachedEntry tests the GetCachedEntry method
func TestGetCachedEntry(t *testing.T) {
	lm := NewLockFileManager(mock.NewMockFileSystem())

	lm.SetLockFile(map[string]LockFileEntry{
		"key1": {CachedFiles: map[string]string{"file1": "cache1"}},
	})

	// Test existing entry
	entry, exists := lm.GetCachedEntry("key1")
	if !exists {
		t.Error("GetCachedEntry should return true for existing key")
	}
	if !reflect.DeepEqual(entry, lm.LockFile()["key1"]) {
		t.Error("GetCachedEntry returned incorrect entry")
	}

	// Test non-existent entry
	_, exists = lm.GetCachedEntry("non-existent")
	if exists {
		t.Error("GetCachedEntry should return false for non-existent key")
	}
}

// TestAddFreshEntry tests the AddFreshEntry method
func TestAddFreshEntry(t *testing.T) {
	lm := NewLockFileManager(mock.NewMockFileSystem())

	entry := LockFileEntry{CachedFiles: map[string]string{"file1": "cache1"}}
	lm.AddFreshEntry("key1", entry)

	if !reflect.DeepEqual(lm.FreshLockFile()["key1"], entry) {
		t.Error("AddFreshEntry did not add the entry correctly")
	}

	// Test overwriting existing entry
	newEntry := LockFileEntry{CachedFiles: map[string]string{"file2": "cache2"}}
	lm.AddFreshEntry("key1", newEntry)

	if !reflect.DeepEqual(lm.FreshLockFile()["key1"], newEntry) {
		t.Error("AddFreshEntry did not overwrite the existing entry")
	}
}

// TestLockFileManagerConcurrency tests concurrent access to LockFileManager
func TestLockFileManagerConcurrency(t *testing.T) {
	lm := NewLockFileManager(mock.NewMockFileSystem())

	// Concurrent reads and writes
	go func() {
		for i := 0; i < 1000; i++ {
			lm.AddFreshEntry(fmt.Sprintf("key%d", i), LockFileEntry{})
		}
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			lm.GetCachedEntry(fmt.Sprintf("key%d", i))
		}
	}()

	// If this test completes without race detector warnings, it passes
}

// Property-based test for LockFileEntry
func TestLockFileEntryProperty(t *testing.T) {
	f := func(key string, files map[string]string) bool {
		lm := NewLockFileManager(mock.NewMockFileSystem())
		entry := LockFileEntry{CachedFiles: files}
		lm.AddFreshEntry(key, entry)
		retrievedEntry, exists := lm.GetCachedEntry(key)
		return !exists && reflect.DeepEqual(lm.FreshLockFile()[key], entry) &&
			!reflect.DeepEqual(retrievedEntry, entry)
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestLockFileManagerEdgeCases(t *testing.T) {
	fs := mock.NewMockFileSystem()
	lm := NewLockFileManager(fs)

	// Test loading a lock file with no read permissions
	fs.Files["generooni.lock"] = &mock.MockFile{Buffer: bytes.NewBuffer([]byte("{}")), ReadOnly: true}
	err := lm.LoadLockFile()
	if err == nil {
		t.Error("LoadLockFile should return an error when file cannot be read")
	}
	if err != os.ErrPermission {
		t.Errorf("Expected os.ErrPermission, got %v", err)
	}

	// Test saving a lock file with no write permissions
	lm.SetFreshLockFile(map[string]LockFileEntry{"key": {}})
	fs.Files["generooni.lock"] = &mock.MockFile{Buffer: bytes.NewBuffer(nil), ReadOnly: true}
	err = lm.SaveFreshLockFile()
	if err == nil {
		t.Error("SaveFreshLockFile should return an error when file cannot be written")
	}

	// Reset the filesystem
	fs = mock.NewMockFileSystem()
	lm = NewLockFileManager(fs)

	// Test with empty key
	lm.AddFreshEntry("", LockFileEntry{})
	if entry, exists := lm.FreshLockFile()[""]; !exists {
		t.Error("LockFileManager should handle empty keys")
	} else if entry.CachedFiles == nil {
		t.Error("LockFileManager should initialize CachedFiles for empty keys")
	}

	// Test with nil CachedFiles
	lm.AddFreshEntry("nilKey", LockFileEntry{CachedFiles: nil})
	if entry, exists := lm.FreshLockFile()["nilKey"]; !exists || entry.CachedFiles == nil {
		t.Error("LockFileManager should handle nil CachedFiles by initializing an empty map")
	}
}
