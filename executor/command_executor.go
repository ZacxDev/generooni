package executor

import (
	"os/exec"
)

// CommandExecutor interface for dependency injection and improved testability
type CommandExecutor interface {
	Execute(name string, arg ...string) ([]byte, error)
}

// RealCommandExecutor implements CommandExecutor interface using actual OS calls
type RealCommandExecutor struct{}

func (RealCommandExecutor) Execute(name string, arg ...string) ([]byte, error) {
	return exec.Command(name, arg...).CombinedOutput()
}
