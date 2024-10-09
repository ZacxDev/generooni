package executor

import (
	"errors"
	"testing"
	"time"

	"github.com/ZacxDev/generooni/fs"
	"github.com/ZacxDev/generooni/fs/mock"
	"github.com/ZacxDev/generooni/target"
)

type mockDAGManager struct {
	addNodeFunc         func(string, []string)
	topologicalSortFunc func() ([]string, error)
}

func (m *mockDAGManager) AddNode(name string, dependencies []string) {
	if m.addNodeFunc != nil {
		m.addNodeFunc(name, dependencies)
	}
}

func (m *mockDAGManager) TopologicalSort() ([]string, error) {
	if m.topologicalSortFunc != nil {
		return m.topologicalSortFunc()
	}
	return nil, nil
}

// MockStatusManager implements the StatusManager interface for testing
type MockStatusManager struct {
	SetStatusFunc         func(string, string)
	UpdateStatusFunc      func(string, string, time.Time, time.Time)
	WaitForCompletionFunc func(string) string
	MarkAsFailedFunc      func(string)
	FailedCountFunc       func() int
}

func (m *MockStatusManager) SetStatus(name, status string) {
	if m.SetStatusFunc != nil {
		m.SetStatusFunc(name, status)
	}
}

func (m *MockStatusManager) UpdateStatus(name, status string, startTime, endTime time.Time) {
	if m.UpdateStatusFunc != nil {
		m.UpdateStatusFunc(name, status, startTime, endTime)
	}
}

func (m *MockStatusManager) WaitForCompletion(name string) string {
	if m.WaitForCompletionFunc != nil {
		return m.WaitForCompletionFunc(name)
	}
	return ""
}

func (m *MockStatusManager) MarkAsFailed(name string) {
	if m.MarkAsFailedFunc != nil {
		m.MarkAsFailedFunc(name)
	}
}

func (m *MockStatusManager) FailedCount() int {
	if m.FailedCountFunc != nil {
		return m.FailedCountFunc()
	}
	return 0
}

type MockLockFileManager struct {
	LoadLockFileFunc      func() error
	SaveFreshLockFileFunc func() error
	GetCachedEntryFunc    func(string) (LockFileEntry, bool)
	AddFreshEntryFunc     func(string, LockFileEntry)
	LockFileFunc          func() map[string]LockFileEntry
	FreshLockFileFunc     func() map[string]LockFileEntry
	FSFunc                func() fs.FileSystem
	SetLockFileFunc       func(map[string]LockFileEntry)
	SetFreshLockFileFunc  func(map[string]LockFileEntry)
	SetFSFunc             func(fs.FileSystem)
}

func (m *MockLockFileManager) LoadLockFile() error {
	if m.LoadLockFileFunc != nil {
		return m.LoadLockFileFunc()
	}
	return nil
}

func (m *MockLockFileManager) SaveFreshLockFile() error {
	if m.SaveFreshLockFileFunc != nil {
		return m.SaveFreshLockFileFunc()
	}
	return nil
}

func (m *MockLockFileManager) GetCachedEntry(key string) (LockFileEntry, bool) {
	if m.GetCachedEntryFunc != nil {
		return m.GetCachedEntryFunc(key)
	}
	return LockFileEntry{}, false
}

func (m *MockLockFileManager) AddFreshEntry(key string, entry LockFileEntry) {
	if m.AddFreshEntryFunc != nil {
		m.AddFreshEntryFunc(key, entry)
	}
}

func (m *MockLockFileManager) LockFile() map[string]LockFileEntry {
	if m.LockFileFunc != nil {
		return m.LockFileFunc()
	}
	return nil
}

func (m *MockLockFileManager) FreshLockFile() map[string]LockFileEntry {
	if m.FreshLockFileFunc != nil {
		return m.FreshLockFileFunc()
	}
	return nil
}

func (m *MockLockFileManager) FS() fs.FileSystem {
	if m.FSFunc != nil {
		return m.FSFunc()
	}
	return nil
}

func (m *MockLockFileManager) SetLockFile(lf map[string]LockFileEntry) {
	if m.SetLockFileFunc != nil {
		m.SetLockFileFunc(lf)
	}
}

func (m *MockLockFileManager) SetFreshLockFile(flf map[string]LockFileEntry) {
	if m.SetFreshLockFileFunc != nil {
		m.SetFreshLockFileFunc(flf)
	}
}

func (m *MockLockFileManager) SetFS(filesystem fs.FileSystem) {
	if m.SetFSFunc != nil {
		m.SetFSFunc(filesystem)
	}
}

type MockCacheManager struct {
	EnsureCacheDirFunc             func() error
	ApplyCachedFileChangesFunc     func(LockFileEntry) error
	CollectAndStoreFileChangesFunc func(*target.FilesystemTarget) (*LockFileEntry, error)
	RestoreFileFunc                func(cachedPath, originalPath string) error
	CacheFileFunc                  func(originalPath string) (string, error)
	FSFunc                         func() fs.FileSystem
	CacheDirFunc                   func() string
	SetFSFunc                      func(fs.FileSystem)
	SetCacheDirFunc                func(string)
}

func (m *MockCacheManager) EnsureCacheDir() error {
	if m.EnsureCacheDirFunc != nil {
		return m.EnsureCacheDirFunc()
	}
	return nil
}

func (m *MockCacheManager) ApplyCachedFileChanges(entry LockFileEntry) error {
	if m.ApplyCachedFileChangesFunc != nil {
		return m.ApplyCachedFileChangesFunc(entry)
	}
	return nil
}

func (m *MockCacheManager) CollectAndStoreFileChanges(target *target.FilesystemTarget) (*LockFileEntry, error) {
	if m.CollectAndStoreFileChangesFunc != nil {
		return m.CollectAndStoreFileChangesFunc(target)
	}
	return nil, nil
}

func (m *MockCacheManager) restoreFile(cachedPath, originalPath string) error {
	if m.RestoreFileFunc != nil {
		return m.RestoreFileFunc(cachedPath, originalPath)
	}
	return nil
}

func (m *MockCacheManager) cacheFile(originalPath string) (string, error) {
	if m.CacheFileFunc != nil {
		return m.CacheFileFunc(originalPath)
	}
	return "", nil
}

func (m *MockCacheManager) FS() fs.FileSystem {
	if m.FSFunc != nil {
		return m.FSFunc()
	}
	return nil
}

func (m *MockCacheManager) CacheDir() string {
	if m.CacheDirFunc != nil {
		return m.CacheDirFunc()
	}
	return ""
}

func (m *MockCacheManager) SetFS(filesystem fs.FileSystem) {
	if m.SetFSFunc != nil {
		m.SetFSFunc(filesystem)
	}
}

func (m *MockCacheManager) SetCacheDir(dir string) {
	if m.SetCacheDirFunc != nil {
		m.SetCacheDirFunc(dir)
	}
}

// MockCommandExecutor implements the CommandExecutor interface for testing
type MockCommandExecutor struct {
	ExecuteFunc func(string, ...string) ([]byte, error)
}

func (m *MockCommandExecutor) Execute(name string, arg ...string) ([]byte, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(name, arg...)
	}
	return nil, nil
}

func TestNewTargetExecutor(t *testing.T) {
	fs := &mock.MockFileSystem{}
	cmdExecutor := &MockCommandExecutor{}
	cacheMgr := &MockCacheManager{}
	lockMgr := &MockLockFileManager{}

	te := NewTargetExecutor(fs, cmdExecutor, cacheMgr, lockMgr)

	if te == nil {
		t.Fatal("NewTargetExecutor returned nil")
	}

	if te.fs != fs {
		t.Error("FileSystem not set correctly")
	}

	if te.cmdExecutor != cmdExecutor {
		t.Error("CommandExecutor not set correctly")
	}

	if te.cacheMgr != cacheMgr {
		t.Error("CacheManager not set correctly")
	}

	if te.lockMgr != lockMgr {
		t.Error("LockFileManager not set correctly")
	}
}

func TestTargetExecutor_Initialize(t *testing.T) {
	cacheMgr := &MockCacheManager{
		EnsureCacheDirFunc: func() error {
			return nil
		},
	}
	lockMgr := &MockLockFileManager{
		LoadLockFileFunc: func() error {
			return nil
		},
	}

	te := &TargetExecutor{
		cacheMgr: cacheMgr,
		lockMgr:  lockMgr,
	}

	err := te.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test error cases
	cacheMgr.EnsureCacheDirFunc = func() error {
		return errors.New("cache dir error")
	}

	err = te.Initialize()
	if err == nil || err.Error() != "failed to create cache directory: cache dir error" {
		t.Errorf("Expected cache dir error, got: %v", err)
	}

	cacheMgr.EnsureCacheDirFunc = func() error { return nil }
	lockMgr.LoadLockFileFunc = func() error {
		return errors.New("lock file error")
	}

	err = te.Initialize()
	if err == nil || err.Error() != "failed to load lock file: lock file error" {
		t.Errorf("Expected lock file error, got: %v", err)
	}
}

func TestTargetExecutor_AddTarget(t *testing.T) {
	mockDAG := &mockDAGManager{}
	mockStatusMgr := &MockStatusManager{}

	te := &TargetExecutor{
		targets:   make(map[string]*target.FilesystemTarget),
		dag:       mockDAG,
		statusMgr: mockStatusMgr,
	}

	addNodeCalled := false
	mockDAG.addNodeFunc = func(name string, deps []string) {
		addNodeCalled = true
		if name != "test_target" {
			t.Errorf("Expected target name 'test_target', got %s", name)
		}
		if len(deps) != 1 || deps[0] != "dep1" {
			t.Errorf("Expected dependencies [dep1], got %v", deps)
		}
	}

	setStatusCalled := false
	mockStatusMgr.SetStatusFunc = func(name, status string) {
		setStatusCalled = true
		if name != "test_target" {
			t.Errorf("Expected target name 'test_target', got %s", name)
		}
		if status != "Queued" {
			t.Errorf("Expected status 'Queued', got %s", status)
		}
	}

	target := &target.FilesystemTarget{
		Name:       "test_target",
		TargetDeps: []string{"dep1"},
	}

	te.AddTarget(target)

	if !addNodeCalled {
		t.Error("AddNode was not called")
	}
	if !setStatusCalled {
		t.Error("SetStatus was not called")
	}
	if _, exists := te.targets["test_target"]; !exists {
		t.Error("Target was not added to the targets map")
	}
}

func TestTargetExecutor_executeTarget(t *testing.T) {
	mockStatusMgr := &MockStatusManager{}
	mockLockMgr := &MockLockFileManager{}
	mockCacheMgr := &MockCacheManager{}
	mockCmdExecutor := &MockCommandExecutor{}

	te := &TargetExecutor{
		targets:     make(map[string]*target.FilesystemTarget),
		statusMgr:   mockStatusMgr,
		lockMgr:     mockLockMgr,
		cacheMgr:    mockCacheMgr,
		cmdExecutor: mockCmdExecutor,
	}

	target := &target.FilesystemTarget{
		Name: "test_target",
		Cmd:  "echo 'test'",
	}
	te.targets["test_target"] = target

	updateStatusCalled := false
	mockStatusMgr.UpdateStatusFunc = func(name, status string, startTime, endTime time.Time) {
		updateStatusCalled = true
		if name != "test_target" {
			t.Errorf("Expected target name 'test_target', got %s", name)
		}
		if status != "Running" && status != "Completed" {
			t.Errorf("Unexpected status: %s", status)
		}
	}

	mockCmdExecutor.ExecuteFunc = func(name string, arg ...string) ([]byte, error) {
		return []byte("test output"), nil
	}

	te.executeTarget("test_target")

	if !updateStatusCalled {
		t.Error("UpdateStatus was not called")
	}

	// Test error case
	mockCmdExecutor.ExecuteFunc = func(name string, arg ...string) ([]byte, error) {
		return nil, errors.New("command error")
	}

	mockStatusMgr.MarkAsFailedFunc = func(name string) {
		if name != "test_target" {
			t.Errorf("Expected target name 'test_target', got %s", name)
		}
	}

	te.executeTarget("test_target")
}

func TestTargetExecutor_waitForDependencies(t *testing.T) {
	mockStatusMgr := &MockStatusManager{}

	te := &TargetExecutor{
		statusMgr: mockStatusMgr,
	}

	target := &target.FilesystemTarget{
		Name:       "test_target",
		TargetDeps: []string{"dep1", "dep2"},
	}

	mockStatusMgr.WaitForCompletionFunc = func(name string) string {
		return "Completed"
	}

	result := te.waitForDependencies("test_target", target)
	if !result {
		t.Error("waitForDependencies should return true when all dependencies are completed")
	}

	mockStatusMgr.WaitForCompletionFunc = func(name string) string {
		if name == "dep2" {
			return "Failed"
		}
		return "Completed"
	}

	updateStatusCalled := false
	mockStatusMgr.UpdateStatusFunc = func(name, status string, startTime, endTime time.Time) {
		updateStatusCalled = true
		if name != "test_target" {
			t.Errorf("Expected target name 'test_target', got %s", name)
		}
		if status != "Skipped" {
			t.Errorf("Expected status 'Skipped', got %s", status)
		}
	}

	result = te.waitForDependencies("test_target", target)
	if result {
		t.Error("waitForDependencies should return false when a dependency fails")
	}
	if !updateStatusCalled {
		t.Error("UpdateStatus was not called")
	}
}

func TestTargetExecutor_getLockfileKey(t *testing.T) {
	te := &TargetExecutor{}

	target := &target.FilesystemTarget{
		Name:      "test_target",
		Cmd:       "echo 'test'",
		InputHash: "input_hash",
		Outputs:   []string{"output1", "output2"},
		IsPartial: false,
	}

	key := te.getLockfileKey(target)
	if key == "" {
		t.Error("getLockfileKey should return a non-empty string for non-partial targets")
	}

	target.IsPartial = true
	key = te.getLockfileKey(target)
	if key != "" {
		t.Error("getLockfileKey should return an empty string for partial targets")
	}
}

func TestTargetExecutor_tryApplyCachedChanges(t *testing.T) {
	mockLockMgr := &MockLockFileManager{}
	mockCacheMgr := &MockCacheManager{}
	mockStatusMgr := &MockStatusManager{}

	te := &TargetExecutor{
		lockMgr:   mockLockMgr,
		cacheMgr:  mockCacheMgr,
		statusMgr: mockStatusMgr,
	}

	mockLockMgr.GetCachedEntryFunc = func(key string) (LockFileEntry, bool) {
		return LockFileEntry{}, true
	}

	mockCacheMgr.ApplyCachedFileChangesFunc = func(entry LockFileEntry) error {
		return nil
	}

	updateStatusCalled := false
	mockStatusMgr.UpdateStatusFunc = func(name, status string, startTime, endTime time.Time) {
		updateStatusCalled = true
		if status != "Completed" {
			t.Errorf("Expected status 'Completed', got %s", status)
		}
	}

	addFreshEntryCalled := false
	mockLockMgr.AddFreshEntryFunc = func(key string, entry LockFileEntry) {
		addFreshEntryCalled = true
	}

	result := te.tryApplyCachedChanges("test_target", "test_key")
	if !result {
		t.Error("tryApplyCachedChanges should return true when cache is applied successfully")
	}
	if !updateStatusCalled {
		t.Error("UpdateStatus was not called")
	}
	if !addFreshEntryCalled {
		t.Error("AddFreshEntry was not called")
	}

	// Test error case
	mockCacheMgr.ApplyCachedFileChangesFunc = func(entry LockFileEntry) error {
		return errors.New("cache error")
	}

	result = te.tryApplyCachedChanges("test_target", "test_key")
	if result {
		t.Error("tryApplyCachedChanges should return false when cache application fails")
	}
}

func TestTargetExecutor_runCommand(t *testing.T) {
	mockCmdExecutor := &MockCommandExecutor{}

	te := &TargetExecutor{
		cmdExecutor: mockCmdExecutor,
	}

	mockCmdExecutor.ExecuteFunc = func(name string, arg ...string) ([]byte, error) {
		return []byte("test output"), nil
	}

	err := te.runCommand("test_target", "echo 'test'")
	if err != nil {
		t.Errorf("runCommand failed: %v", err)
	}

	// Test error case
	mockCmdExecutor.ExecuteFunc = func(name string, arg ...string) ([]byte, error) {
		return nil, errors.New("command error")
	}

	err = te.runCommand("test_target", "echo 'test'")
	if err == nil {
		t.Error("runCommand should return an error when command execution fails")
	}
}

func TestTargetExecutor_handleExecutionFailure(t *testing.T) {
	mockStatusMgr := &MockStatusManager{}

	te := &TargetExecutor{
		statusMgr: mockStatusMgr,
	}

	target := &target.FilesystemTarget{
		Name:         "test_target",
		AllowFailure: false,
	}

	markAsFailedCalled := false
	mockStatusMgr.MarkAsFailedFunc = func(name string) {
		markAsFailedCalled = true
		if name != "test_target" {
			t.Errorf("Expected target name 'test_target', got %s", name)
		}
	}

	te.handleExecutionFailure("test_target", target)
	if !markAsFailedCalled {
		t.Error("MarkAsFailed was not called")
	}

	// Test allow failure case
	target.AllowFailure = true

	updateStatusCalled := false
	mockStatusMgr.UpdateStatusFunc = func(name, status string, startTime, endTime time.Time) {
		updateStatusCalled = true
		if name != "test_target" {
			t.Errorf("Expected target name 'test_target', got %s", name)
		}
		if status != "Completed" {
			t.Errorf("Expected status 'Completed', got %s", status)
		}
	}

	te.handleExecutionFailure("test_target", target)
	if !updateStatusCalled {
		t.Error("UpdateStatus was not called")
	}
}

func TestTargetExecutor_handleCacheUpdate(t *testing.T) {
	mockCacheMgr := &MockCacheManager{}
	mockLockMgr := &MockLockFileManager{}

	te := &TargetExecutor{
		cacheMgr: mockCacheMgr,
		lockMgr:  mockLockMgr,
	}

	tar := &target.FilesystemTarget{
		Name: "test_target",
	}

	mockCacheMgr.CollectAndStoreFileChangesFunc = func(target *target.FilesystemTarget) (*LockFileEntry, error) {
		return &LockFileEntry{}, nil
	}

	addFreshEntryCalled := false
	mockLockMgr.AddFreshEntryFunc = func(key string, entry LockFileEntry) {
		addFreshEntryCalled = true
		if key != "test_key" {
			t.Errorf("Expected key 'test_key', got %s", key)
		}
	}

	te.handleCacheUpdate("test_target", tar, "test_key")
	if !addFreshEntryCalled {
		t.Error("AddFreshEntry was not called")
	}

	// Test error case
	mockCacheMgr.CollectAndStoreFileChangesFunc = func(target *target.FilesystemTarget) (*LockFileEntry, error) {
		return nil, errors.New("cache error")
	}

	te.handleCacheUpdate("test_target", tar, "test_key")
	// Ensure the function doesn't panic on error
}
