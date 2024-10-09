package executor

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/ZacxDev/generooni/fs"
	"github.com/ZacxDev/generooni/target"
	"github.com/pkg/errors"
)

type TargetExecutor struct {
	targets     map[string]*target.FilesystemTarget
	dag         DAGManager
	statusMgr   StatusManager
	lockMgr     LockFileManager
	cacheMgr    CacheManager
	cmdExecutor CommandExecutor
	fs          fs.FileSystem
}

func NewTargetExecutor(
	fs fs.FileSystem,
	cmdExecutor CommandExecutor,
	cacheMgr CacheManager,
	lockMgr LockFileManager,
) *TargetExecutor {
	return &TargetExecutor{
		targets:     make(map[string]*target.FilesystemTarget),
		dag:         NewDAGManager(),
		statusMgr:   NewStatusManager(),
		lockMgr:     lockMgr,
		cacheMgr:    cacheMgr,
		cmdExecutor: cmdExecutor,
		fs:          fs,
	}
}

func (te *TargetExecutor) Initialize() error {
	if err := te.cacheMgr.EnsureCacheDir(); err != nil {
		return errors.Wrap(err, "failed to create cache directory")
	}

	if err := te.lockMgr.LoadLockFile(); err != nil {
		return errors.Wrap(err, "failed to load lock file")
	}

	return nil
}

func (te *TargetExecutor) AddTarget(target *target.FilesystemTarget) {
	te.targets[target.Name] = target
	te.dag.AddNode(target.Name, target.TargetDeps)
	te.statusMgr.SetStatus(target.Name, "Queued")
}

func (te *TargetExecutor) ExecuteTargets() error {
	order, err := te.dag.TopologicalSort()
	if err != nil {
		return errors.Wrap(err, "failed to perform topological sort")
	}

	var wg sync.WaitGroup
	for _, name := range order {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			te.executeTarget(name)
		}(name)
	}

	wg.Wait()

	if te.statusMgr.FailedCount() > 0 {
		return errors.Errorf("execution failed for %d target(s)", te.statusMgr.FailedCount())
	}

	return te.lockMgr.SaveFreshLockFile()
}

func (te *TargetExecutor) executeTarget(name string) {
	target := te.targets[name]

	if !te.waitForDependencies(name, target) {
		return
	}

	te.statusMgr.UpdateStatus(name, "Running", time.Now(), time.Time{})

	lockfileKey := te.getLockfileKey(target)
	if !target.IsPartial && te.tryApplyCachedChanges(name, lockfileKey) {
		return
	}

	if err := te.runCommand(name, target.Cmd); err != nil {
		te.handleExecutionFailure(name, target)
		return
	}

	te.statusMgr.UpdateStatus(name, "Completed", time.Time{}, time.Now())

	if lockfileKey != "" {
		te.handleCacheUpdate(name, target, lockfileKey)
	}

	fmt.Printf("[%s] completed\n", name)
}

func (te *TargetExecutor) waitForDependencies(name string, target *target.FilesystemTarget) bool {
	for _, dep := range target.TargetDeps {
		status := te.statusMgr.WaitForCompletion(dep)
		if status == "Failed" {
			te.statusMgr.UpdateStatus(name, "Skipped", time.Time{}, time.Now())
			fmt.Printf("[%s] skipped due to dependency failure\n", name)
			return false
		}
	}
	return true
}

func (te *TargetExecutor) getLockfileKey(target *target.FilesystemTarget) string {
	if target.IsPartial {
		return ""
	}

	h := md5.New()
	io.WriteString(h, target.Cmd)
	io.WriteString(h, target.InputHash)
	for _, pattern := range target.Outputs {
		io.WriteString(h, pattern)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (te *TargetExecutor) tryApplyCachedChanges(name, lockfileKey string) bool {
	if lockfileKey == "" {
		return false
	}

	entry, hasCachedChanges := te.lockMgr.GetCachedEntry(lockfileKey)
	if !hasCachedChanges {
		return false
	}

	err := te.cacheMgr.ApplyCachedFileChanges(entry)
	if err != nil {
		log.Printf("Error applying cached file changes for target %s. Continuing with job execution: %v", name, err)
		return false
	}

	te.statusMgr.UpdateStatus(name, "Completed", time.Time{}, time.Now())
	te.lockMgr.AddFreshEntry(lockfileKey, entry)
	fmt.Printf("[%s] completed [cached]\n", name)
	return true
}

func (te *TargetExecutor) runCommand(name, cmd string) error {
	output, err := te.cmdExecutor.Execute("sh", "-c", cmd)
	if err != nil {
		log.Printf("Error executing target %s: %v", name, err)
		return err
	}
	fmt.Printf("[%s] %s", name, string(output))
	return nil
}

func (te *TargetExecutor) handleExecutionFailure(name string, target *target.FilesystemTarget) {
	if !target.AllowFailure {
		te.statusMgr.MarkAsFailed(name)
		fmt.Printf("[%s] failed\n", name)
	} else {
		te.statusMgr.UpdateStatus(name, "Completed", time.Time{}, time.Now())
		fmt.Printf("[%s] failed, but continuing due to allow_failure flag\n", name)
	}
}

func (te *TargetExecutor) handleCacheUpdate(name string, target *target.FilesystemTarget, lockfileKey string) {
	entry, err := te.cacheMgr.CollectAndStoreFileChanges(target)
	if err != nil {
		log.Printf("Error collecting file changes for target %s: %v", name, err)
		return
	}

	if entry != nil {
		te.lockMgr.AddFreshEntry(lockfileKey, *entry)
	}
}

func (te *TargetExecutor) MapDependencies() error {
	dependencyMap := make(map[string][]string)

	for name, target := range te.targets {
		var deps []string
		for _, pattern := range target.Dependencies {
			matches, err := te.fs.DoublestarGlob(pattern)
			if err != nil {
				return errors.Wrapf(err, "error expanding glob pattern %s for target %s", pattern, name)
			}
			deps = append(deps, matches...)
		}

		dependencyMap[name] = deps
	}

	content := fmt.Sprintf("filesystem_target_dependency_map = %#v", dependencyMap)

	err := te.fs.WriteFile("generooni-deps.star", []byte(content), 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write generooni-deps.star file")
	}

	fmt.Println("Generated generooni-deps.star file successfully.")
	return nil
}
