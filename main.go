package main

import (
	"bufio"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"go.starlark.net/starlark"
)

type FilesystemTarget struct {
	Name         string
	Cmd          string
	Outputs      []string
	Dependencies []string
	TargetDeps   []string
	InputHash    string
	OutputHash   string
}

type ExecutionStatus struct {
	Status    string
	StartTime time.Time
	EndTime   time.Time
}

type LockFileEntry struct {
	CachedFiles map[string]string
}

type TargetExecutor struct {
	Targets       map[string]*FilesystemTarget
	DAG           map[string][]string
	mu            sync.Mutex
	FileChanges   map[string]FileChangeInfo
	fileChangesMu sync.Mutex
	StatusMap     map[string]*ExecutionStatus
	failedTargets []string
	failMu        sync.Mutex
	wg            sync.WaitGroup
	LockFile      map[string]LockFileEntry
	CacheDir      string
}

type FileChangeInfo struct {
	Content []byte
}

func NewTargetExecutor() *TargetExecutor {
	return &TargetExecutor{
		Targets:   make(map[string]*FilesystemTarget),
		DAG:       make(map[string][]string),
		StatusMap: make(map[string]*ExecutionStatus),
		LockFile:  make(map[string]LockFileEntry),
		CacheDir:  ".generooni-cache",
	}
}

func (te *TargetExecutor) AddTarget(target *FilesystemTarget) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.Targets[target.Name] = target
	te.DAG[target.Name] = target.TargetDeps
	te.StatusMap[target.Name] = &ExecutionStatus{
		Status: "Queued",
	}
}

func (te *TargetExecutor) ExecuteTargets() error {
	order, err := te.topologicalSort()
	if err != nil {
		return err
	}

	for _, name := range order {
		te.wg.Add(1)
		go func(name string) {
			defer te.wg.Done()
			te.executeTarget(name)
		}(name)
	}

	te.wg.Wait()

	te.failMu.Lock()
	failedCount := len(te.failedTargets)
	te.failMu.Unlock()

	if failedCount > 0 {
		return fmt.Errorf("execution failed for %d target(s)", failedCount)
	}

	return te.saveLockFile()
}

func (te *TargetExecutor) executeTarget(name string) {
	target := te.Targets[name]
	te.mu.Lock()
	status := te.StatusMap[name]
	te.mu.Unlock()

	// Wait for dependencies
	for _, dep := range target.TargetDeps {
		te.mu.Lock()
		depStatus := te.StatusMap[dep]
		te.mu.Unlock()
		for depStatus.Status != "Completed" && depStatus.Status != "Failed" {
			time.Sleep(100 * time.Millisecond)
			te.mu.Lock()
			depStatus = te.StatusMap[dep]
			te.mu.Unlock()
		}
		if depStatus.Status == "Failed" {
			te.mu.Lock()
			status.Status = "Skipped"
			status.EndTime = time.Now()
			te.mu.Unlock()
			fmt.Printf("[%s] skipped due to dependency failure\n", name)
			return
		}
	}

	te.mu.Lock()
	status.Status = "Running"
	status.StartTime = time.Now()
	te.mu.Unlock()

	lockfileKey, err := te.calculateLockfileKey(target)
	if err != nil {
		log.Printf("Error calculating lockfile key for target %s: %v", name, err)
		te.mu.Lock()
		status.Status = "Failed"
		status.EndTime = time.Now()
		te.mu.Unlock()
		return
	}

	te.fileChangesMu.Lock()
	_, hasCachedChanges := te.LockFile[lockfileKey]
	te.fileChangesMu.Unlock()

	if hasCachedChanges {
		err := te.applyCachedFileChanges(lockfileKey)
		if err == nil {
			te.mu.Lock()
			status.Status = "Completed"
			status.EndTime = time.Now()
			te.mu.Unlock()

			fmt.Printf("[%s] completed [cached]\n", name)
			return
		} else {
			log.Printf("Error applying cached file changes for target %s. continuing with job executtion.\n %v.", name, err)
		}
	}

	// Execute the actual command
	cmd := exec.Command("sh", "-c", target.Cmd)

	// Create pipes for stdout and stderr
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("Error starting command for target %s: %v", name, err)
		te.mu.Lock()
		status.Status = "Failed"
		status.EndTime = time.Now()
		te.mu.Unlock()
		return
	}

	// Start goroutines to read from stdout and stderr
	go te.readAndLogOutput(name, stdout, os.Stdout)
	go te.readAndLogOutput(name, stderr, os.Stderr)

	if err := cmd.Wait(); err != nil {
		log.Printf("Error executing target %s: %v", name, err)
		te.mu.Lock()
		status.Status = "Failed"
		status.EndTime = time.Now()
		te.mu.Unlock()

		te.failMu.Lock()
		te.failedTargets = append(te.failedTargets, name)
		te.failMu.Unlock()

		fmt.Printf("[%s] failed\n", name)
	} else {
		te.mu.Lock()
		status.Status = "Completed"
		status.EndTime = time.Now()
		te.mu.Unlock()

		if err := te.collectAndStoreFileChanges(target, lockfileKey); err != nil {
			log.Printf("Error collecting file changes for target %s: %v", name, err)
		}

		fmt.Printf("[%s] completed\n", name)
	}
}

func (te *TargetExecutor) readAndLogOutput(name string, pipe io.Reader, output io.Writer) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Fprintf(output, "[%s] %s\n", name, scanner.Text())
	}
}

func (te *TargetExecutor) calculateLockfileKey(target *FilesystemTarget) (string, error) {
	h := md5.New()

	// Hash the job's command
	io.WriteString(h, target.Cmd)

	// Hash the job's inputs
	for _, pattern := range target.Dependencies {
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return "", fmt.Errorf("error expanding glob pattern %s: %v", pattern, err)
		}
		for _, match := range matches {
			io.WriteString(h, match)
		}
	}

	for _, pattern := range target.Outputs {
		io.WriteString(h, pattern)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (te *TargetExecutor) topologicalSort() ([]string, error) {
	visited := make(map[string]bool)
	var order []string

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		visited[name] = true

		for _, dep := range te.DAG[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		order = append(order, name)
		return nil
	}

	for name := range te.Targets {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	for i := 0; i < len(order)/2; i++ {
		j := len(order) - 1 - i
		order[i], order[j] = order[j], order[i]
	}

	return order, nil
}

func parseStarlarkConfig(filename string) (map[string]*FilesystemTarget, error) {
	thread := &starlark.Thread{Name: "generooni"}
	globals, err := starlark.ExecFile(thread, filename, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error executing Starlark script: %v", err)
	}

	configValue, ok := globals["config"]
	if !ok {
		return nil, fmt.Errorf("global 'config' object not found in Starlark config")
	}

	configDict, ok := configValue.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("global 'config' object is not a dictionary")
	}

	targets := make(map[string]*FilesystemTarget)

	for _, item := range configDict.Items() {
		name := item.Index(0).(starlark.String).GoString()
		value := item.Index(1)
		if dict, ok := value.(*starlark.Dict); ok {
			target, err := parseTarget(name, dict)
			if err != nil {
				return nil, fmt.Errorf("error parsing target %s: %v", name, err)
			}
			targets[name] = target
		}
	}

	return targets, nil
}

func parseTarget(name string, dict *starlark.Dict) (*FilesystemTarget, error) {
	target := &FilesystemTarget{Name: name}

	if cmd, ok, err := getStringValue(dict, "cmd"); err != nil {
		return nil, err
	} else if ok {
		target.Cmd = cmd
	}

	if outputs, ok, err := getStringList(dict, "outputs"); err != nil {
		return nil, err
	} else if ok {
		target.Outputs = outputs
	}

	if deps, ok, err := getStringList(dict, "dependencies"); err != nil {
		return nil, err
	} else if ok {
		target.Dependencies = deps
	}

	if targetDeps, ok, err := getStringList(dict, "target_deps"); err != nil {
		return nil, err
	} else if ok {
		target.TargetDeps = targetDeps
	}

	return target, nil
}

func getStringValue(dict *starlark.Dict, key string) (string, bool, error) {
	value, found, err := dict.Get(starlark.String(key))
	if err != nil || !found {
		return "", false, err
	}

	strValue, ok := value.(starlark.String)
	if !ok {
		return "", false, fmt.Errorf("expected string for key %s, got %T", key, value)
	}

	return strValue.GoString(), true, nil
}

func getStringList(dict *starlark.Dict, key string) ([]string, bool, error) {
	value, found, err := dict.Get(starlark.String(key))
	if err != nil || !found {
		return nil, false, err
	}

	list, ok := value.(*starlark.List)
	if !ok {
		return nil, false, fmt.Errorf("expected list for key %s, got %T", key, value)
	}

	var result []string
	iter := list.Iterate()
	defer iter.Done()
	var x starlark.Value
	for iter.Next(&x) {
		str, ok := x.(starlark.String)
		if !ok {
			return nil, false, fmt.Errorf("expected string in list for key %s, got %T", key, x)
		}
		result = append(result, str.GoString())
	}

	return result, true, nil
}

func (te *TargetExecutor) saveLockFile() error {
	lockFile, err := os.Create("generooni.lock")
	if err != nil {
		return err
	}
	defer lockFile.Close()

	encoder := json.NewEncoder(lockFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(te.LockFile)
}

func (te *TargetExecutor) loadLockFile() error {
	lockFile, err := os.Open("generooni.lock")
	if err != nil {
		if os.IsNotExist(err) {
			return nil // It's okay if the lock file doesn't exist yet
		}
		return err
	}
	defer lockFile.Close()

	decoder := json.NewDecoder(lockFile)
	return decoder.Decode(&te.LockFile)
}

func (te *TargetExecutor) collectAndStoreFileChanges(target *FilesystemTarget, lockfileKey string) error {
	entry := LockFileEntry{
		CachedFiles: make(map[string]string),
	}

	for _, pattern := range target.Outputs {
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return fmt.Errorf("error expanding glob pattern %s: %v", pattern, err)
		}
		for _, match := range matches {
			err := filepath.WalkDir(match, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					cachedPath, err := te.cacheFile(path)
					if err != nil {
						return fmt.Errorf("error caching file %s: %v", path, err)
					}
					entry.CachedFiles[path] = cachedPath
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("error walking directory %s: %v", match, err)
			}
		}
	}

	te.fileChangesMu.Lock()
	defer te.fileChangesMu.Unlock()
	te.LockFile[lockfileKey] = entry

	return nil
}

func (te *TargetExecutor) cacheFile(originalPath string) (string, error) {
	content, err := os.ReadFile(originalPath)
	if err != nil {
		return "", fmt.Errorf("error reading file %s: %v", originalPath, err)
	}

	hash := sha256.Sum256(content)
	hashString := hex.EncodeToString(hash[:])

	cachedPath := filepath.Join(te.CacheDir, hashString)
	if err := os.MkdirAll(filepath.Dir(cachedPath), 0755); err != nil {
		return "", fmt.Errorf("error creating cache directory: %v", err)
	}

	if err := os.WriteFile(cachedPath, content, 0644); err != nil {
		return "", fmt.Errorf("error writing cached file: %v", err)
	}

	return cachedPath, nil
}

func (te *TargetExecutor) applyCachedFileChanges(lockfileKey string) error {
	te.fileChangesMu.Lock()
	entry, ok := te.LockFile[lockfileKey]
	te.fileChangesMu.Unlock()

	if !ok {
		return fmt.Errorf("no cached entry found for key %s", lockfileKey)
	}

	missingFiles := te.verifyCacheIntegrity(entry)
	if len(missingFiles) > 0 {
		fmt.Printf("Warning: Some cached files are missing. Rebuilding target.\n")
		for _, file := range missingFiles {
			fmt.Printf("  Missing: %s\n", file)
		}
		return fmt.Errorf("cache integrity check failed")
	}

	for originalPath, cachedPath := range entry.CachedFiles {
		if err := te.restoreFile(cachedPath, originalPath); err != nil {
			return fmt.Errorf("error restoring file %s: %v", originalPath, err)
		}
	}

	return nil
}

func (te *TargetExecutor) verifyCacheIntegrity(entry LockFileEntry) []string {
	var missingFiles []string
	for _, cachedPath := range entry.CachedFiles {
		if _, err := os.Stat(cachedPath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, cachedPath)
		}
	}
	return missingFiles
}

func (te *TargetExecutor) restoreFile(cachedPath, originalPath string) error {
	content, err := os.ReadFile(cachedPath)
	if err != nil {
		return fmt.Errorf("error reading cached file %s: %v", cachedPath, err)
	}

	originalInfo, err := os.Stat(originalPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error stating original file %s: %v", originalPath, err)
	}

	var originalMode os.FileMode = 0644
	var originalPermissions os.FileMode
	fileExists := err == nil

	if fileExists {
		originalMode = originalInfo.Mode()
		originalPermissions = originalMode.Perm()
	}

	if err := os.MkdirAll(filepath.Dir(originalPath), 0755); err != nil {
		return fmt.Errorf("error creating directory for %s: %v", originalPath, err)
	}

	// Check if the file is writable
	if fileExists && originalPermissions&0200 == 0 {
		// File is not writable, so make it writable
		if err := os.Chmod(originalPath, originalPermissions|0200); err != nil {
			return fmt.Errorf("error making file writable %s: %v", originalPath, err)
		}
		// Ensure we restore the original permissions after writing
		defer func() {
			if err := os.Chmod(originalPath, originalPermissions); err != nil {
				fmt.Printf("Warning: error restoring original permissions for %s: %v\n", originalPath, err)
			}
		}()
	}

	if err := os.WriteFile(originalPath, content, originalMode); err != nil {
		return fmt.Errorf("error writing restored file %s: %v", originalPath, err)
	}

	return nil
}

func main() {
	flag.Parse()

	executor := NewTargetExecutor()

	if err := os.MkdirAll(executor.CacheDir, 0755); err != nil {
		log.Fatalf("Error creating cache directory: %v", err)
	}

	if err := executor.loadLockFile(); err != nil {
		log.Fatalf("Error loading lock file: %v", err)
	}

	targets, err := parseStarlarkConfig("generooni.star")
	if err != nil {
		log.Fatalf("Error parsing Starlark config: %v", err)
	}

	for _, target := range targets {
		executor.AddTarget(target)
	}

	if err := executor.ExecuteTargets(); err != nil {
		log.Fatalf("Error executing targets: %v", err)
	}

	fmt.Println("All targets completed successfully.")
}
