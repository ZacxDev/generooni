// executor/status_manager.go

package executor

import (
	"sync"
	"time"
)

type ExecutionStatus struct {
	Status    string
	StartTime time.Time
	EndTime   time.Time
}

type StatusManager interface {
	SetStatus(name, status string)
	UpdateStatus(name, status string, startTime, endTime time.Time)
	WaitForCompletion(name string) string
	MarkAsFailed(name string)
	FailedCount() int
}

type statusManager struct {
	statusMap     map[string]*ExecutionStatus
	failedTargets []string
	mu            sync.Mutex
}

func NewStatusManager() StatusManager {
	return &statusManager{
		statusMap: make(map[string]*ExecutionStatus),
	}
}

func (sm *statusManager) SetStatus(name, status string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.statusMap[name] = &ExecutionStatus{Status: status}
}

func (sm *statusManager) UpdateStatus(name, status string, startTime, endTime time.Time) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, exists := sm.statusMap[name]; !exists {
		sm.statusMap[name] = &ExecutionStatus{}
	}
	sm.statusMap[name].Status = status
	if !startTime.IsZero() {
		sm.statusMap[name].StartTime = startTime
	}
	if !endTime.IsZero() {
		sm.statusMap[name].EndTime = endTime
	}
}

func (sm *statusManager) WaitForCompletion(name string) string {
	for {
		sm.mu.Lock()
		status := sm.statusMap[name].Status
		sm.mu.Unlock()
		if status == "Completed" || status == "Failed" {
			return status
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (sm *statusManager) MarkAsFailed(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.failedTargets = append(sm.failedTargets, name)
	sm.statusMap[name].Status = "Failed"
}

func (sm *statusManager) FailedCount() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return len(sm.failedTargets)
}
