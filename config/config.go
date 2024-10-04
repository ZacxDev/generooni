package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ZacxDev/generooni/target"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

// ModuleCache is used to store loaded Starlark modules
type ModuleCache struct {
	modules map[string]starlark.StringDict
	mutex   sync.RWMutex
}

// NewModuleCache creates a new ModuleCache
func NewModuleCache() *ModuleCache {
	return &ModuleCache{
		modules: make(map[string]starlark.StringDict),
	}
}

// Get retrieves a module from the cache
func (mc *ModuleCache) Get(key string) (starlark.StringDict, bool) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	module, ok := mc.modules[key]
	return module, ok
}

// Set stores a module in the cache
func (mc *ModuleCache) Set(key string, module starlark.StringDict) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.modules[key] = module
}

// LoadModule is a custom load function for Starlark that implements caching
func LoadModule(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	cache := thread.Local("moduleCache").(*ModuleCache)

	// Check if the module is already cached
	if cachedModule, ok := cache.Get(module); ok {
		return cachedModule, nil
	}

	// If not cached, load the module
	filename := module
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(filepath.Dir(thread.Name), filename)
	}

	globals, err := starlark.ExecFile(thread, filename, nil, nil)
	if err != nil {
		return nil, err
	}

	// Cache the loaded module
	cache.Set(module, globals)

	return globals, nil
}

func ParseStarlarkConfig(filename string) (map[string]*target.FilesystemTarget, error) {
	cache := NewModuleCache()
	thread := &starlark.Thread{
		Name: filename,
		Load: LoadModule,
	}
	thread.SetLocal("moduleCache", cache)

	globals, err := starlark.ExecFile(thread, filename, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute Starlark script")
	}

	configValue, ok := globals["config"]
	if !ok {
		return nil, errors.New("global 'config' object not found in Starlark config")
	}

	configDict, ok := configValue.(*starlark.Dict)
	if !ok {
		return nil, errors.New("global 'config' object is not a dictionary")
	}

	targets := make(map[string]*target.FilesystemTarget)

	for _, item := range configDict.Items() {
		name := item.Index(0).(starlark.String).GoString()
		value := item.Index(1)
		if dict, ok := value.(*starlark.Dict); ok {
			target, err := parseTarget(name, dict)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse target %s", name)
			}

			targets[name] = target
		}
	}

	return targets, nil
}

func parseTarget(name string, dict *starlark.Dict) (*target.FilesystemTarget, error) {
	target := &target.FilesystemTarget{Name: name}

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

	if depsSync, ok, err := getStringList(dict, "dependency_sync_patterns"); err != nil {
		return nil, err
	} else if ok {
		target.DependencySyncPatterns = depsSync
	}

	if targetDeps, ok, err := getStringList(dict, "target_deps"); err != nil {
		return nil, err
	} else if ok {
		target.TargetDeps = targetDeps
	}

	if isPartial, ok, err := getBooleanValue(dict, "is_partial"); err != nil {
		return nil, err
	} else if ok {
		target.IsPartial = isPartial
	}

	// Calculate input hash based on dependencies content
	inputHash, err := calculateInputHash(target.Dependencies)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate input hash")
	}
	target.InputHash = inputHash

	return target, nil
}

func calculateInputHash(dependencies []string) (string, error) {
	h := sha256.New()
	for _, dep := range dependencies {
		content, err := os.ReadFile(dep)
		if err != nil && !os.IsNotExist(err) {
			return "", errors.Wrapf(err, "failed to read dependency file: %s", dep)
		}
		h.Write(content)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func getBooleanValue(dict *starlark.Dict, key string) (bool, bool, error) {
	value, found, err := dict.Get(starlark.String(key))
	if err != nil || !found {
		return false, false, err
	}

	strValue, ok := value.(starlark.Bool)
	if !ok {
		return false, false, fmt.Errorf("expected string for key %s, got %T", key, value)
	}

	return bool(strValue), true, nil
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
