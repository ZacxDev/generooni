package main

import (
	"bufio"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"golang.org/x/exp/slices"

	tea "github.com/charmbracelet/bubbletea"
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

type TargetStatus struct {
	*ExecutionStatus
	LogLines []string
}

type FileChangeInfo struct {
	Content []byte
	ModTime time.Time
}

type TargetExecutor struct {
	Targets       map[string]*FilesystemTarget
	DAG           map[string][]string
	mu            sync.Mutex
	FileChanges   map[string]FileChangeInfo
	fileChangesMu sync.Mutex
	StatusMap     map[string]*TargetStatus
	LockFile      map[string]map[string]FileChangeInfo
}

type ExecutionStatus struct {
	Status        string
	FileChanges   int
	QueuePosition int
	Skipped       bool
	StartTime     time.Time
	EndTime       time.Time
}

// Update the saveLockFile function
func (te *TargetExecutor) saveLockFile() error {
	lockFile, err := os.Create(".generooni.lock")
	if err != nil {
		return err
	}
	defer lockFile.Close()

	encoder := json.NewEncoder(lockFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(te.LockFile)
}

// Update the NewTargetExecutor function
func NewTargetExecutor() *TargetExecutor {
	return &TargetExecutor{
		Targets:   make(map[string]*FilesystemTarget),
		DAG:       make(map[string][]string),
		LockFile:  make(map[string]map[string]FileChangeInfo),
		StatusMap: make(map[string]*TargetStatus),
	}
}

// Add a function to load the lock file
func (te *TargetExecutor) loadLockFile() error {
	lockFile, err := os.Open(".generooni.lock")
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

// Modify the AddTarget function to initialize the status for each target
func (te *TargetExecutor) AddTarget(target *FilesystemTarget) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.Targets[target.Name] = target
	te.DAG[target.Name] = target.TargetDeps
	te.StatusMap[target.Name] = &TargetStatus{
		ExecutionStatus: &ExecutionStatus{
			Status: "Queued",
		},
		LogLines: make([]string, 0),
	}
}

func (te *TargetExecutor) ExecuteTargets(logOut bool) error {
	startTime := time.Now()
	order, err := te.topologicalSort()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	statusMap := make(map[string]*ExecutionStatus)
	statusChan := make(chan struct{})

	for _, name := range order {
		statusMap[name] = &ExecutionStatus{Status: "Queued", FileChanges: 0, QueuePosition: slices.Index(order, name), Skipped: false}
	}

	if !logOut {
		go te.reportStatus(statusChan)
	}

	for _, name := range order {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			te.executeTarget(name, statusMap, startTime, statusChan, logOut)
		}(name)
	}

	wg.Wait()
	close(statusChan)

	return te.saveLockFile()
}

// New function to calculate the lockfile key for a job
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

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func hashPathRecursively(h io.Writer, path string) error {
	return filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return hashFile(h, path)
		}
		return nil
	})
}

func hashFile(h io.Writer, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	return nil
}

// New function to collect and store file changes
func (te *TargetExecutor) collectAndStoreFileChanges(target *FilesystemTarget, lockfileKey string) error {
	changes := make(map[string]FileChangeInfo)

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
					info, err := d.Info()
					if err != nil {
						return fmt.Errorf("error getting file info for %s: %v", path, err)
					}
					content, err := os.ReadFile(path)
					if err != nil {
						return fmt.Errorf("error reading file %s: %v", path, err)
					}
					encodedContent := base64.StdEncoding.EncodeToString(content)
					changes[path] = FileChangeInfo{
						Content: []byte(encodedContent),
						ModTime: info.ModTime(),
					}
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
	te.LockFile[lockfileKey] = changes

	return nil
}

// New function to apply cached file changes
func (te *TargetExecutor) applyCachedFileChanges(changes map[string]FileChangeInfo) error {
	for path, info := range changes {
		fileExists := true

		fileInfo, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			fileExists = false
		}

		originalMode := fs.FileMode(0666)
		if fileExists && fileInfo != nil {
			originalMode = fileInfo.Mode()

			// Make the file writable
			err = os.Chmod(path, 0666)
			if err != nil {
				fmt.Println("Error changing file permissions:", err)
				return err
			}
		}

		decodedContent, err := base64.StdEncoding.DecodeString(string(info.Content))
		if err != nil {
			return errors.WithStack(err)
		}

		if err := os.WriteFile(path, decodedContent, 0666); err != nil {
			return err
		}
		if err := os.Chtimes(path, info.ModTime, info.ModTime); err != nil {
			return err
		}

		err = os.Chmod(path, originalMode)
		if err != nil {
			fmt.Println("Error restoring original file permissions:", err)
			return err
		}
	}
	return nil
}

func (te *TargetExecutor) readAndLogOutput(name string, pipe io.Reader, output io.Writer) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Fprintf(output, "[%s] %s\n", name, scanner.Text())
	}
}

func (te *TargetExecutor) executeTarget(name string, statusMap map[string]*ExecutionStatus, startTime time.Time, done <-chan struct{}, logOut bool) {
	target := te.Targets[name]
	te.mu.Lock()
	status := te.StatusMap[name]
	te.mu.Unlock()

	// Wait for dependencies
	for _, dep := range target.TargetDeps {
		te.mu.Lock()
		depStatus := te.StatusMap[dep]
		te.mu.Unlock()
		for depStatus.Status != "Completed" && !depStatus.Skipped {
			time.Sleep(100 * time.Millisecond)
			te.mu.Lock()
			depStatus = te.StatusMap[dep]
			te.mu.Unlock()
		}
	}

	te.mu.Lock()
	status.Status = "Running"
	status.QueuePosition = 0
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

	// Check for cached file changes
	te.fileChangesMu.Lock()
	cachedChanges, hasCachedChanges := te.LockFile[lockfileKey]
	te.fileChangesMu.Unlock()

	if hasCachedChanges {
		// Apply cached file changes and skip running the job
		if err := te.applyCachedFileChanges(cachedChanges); err != nil {
			msg := fmt.Sprintf("Error applying cached file changes for target %s: %v", name, err)

			f, err := tea.LogToFile("debug.log", "debug")
			if err != nil {
				fmt.Println("fatal:", msg)
				os.Exit(1)
			}
			defer f.Close()
			f.WriteString(msg)

			te.mu.Lock()
			status.Status = "Failed"
			status.EndTime = time.Now()
			te.mu.Unlock()
			return
		}
		te.mu.Lock()
		status.Status = "Completed [cache]"
		status.Skipped = true
		status.EndTime = time.Now()
		te.mu.Unlock()
		return
	}

	// Execute the actual command
	cmd := exec.Command("sh", "-c", target.Cmd)

	go func() {
		<-done
		cmd.Process.Signal(os.Kill)
	}()

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
	if logOut {
		go te.readAndLogOutput(name, stdout, os.Stdout)
		go te.readAndLogOutput(name, stderr, os.Stderr)
	} else {
		go te.readAndUpdateLogs(name, stdout)
		go te.readAndUpdateLogs(name, stderr)
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("Error executing target %s: %v", name, err)
		te.mu.Lock()
		status.Status = "Failed"
		status.EndTime = time.Now()
		te.mu.Unlock()
	} else {
		te.mu.Lock()
		status.Status = "Completed"
		status.EndTime = time.Now()
		te.mu.Unlock()

		if err := te.collectAndStoreFileChanges(target, lockfileKey); err != nil {
			log.Printf("Error collecting file changes for target %s: %v", name, err)
		}
	}
}

type model struct {
	viewport      viewport.Model
	targets       map[string]*FilesystemTarget
	statusMap     map[string]*TargetStatus
	done          bool
	mu            sync.Mutex
	selectedIdx   int
	logView       viewport.Model
	showingLogs   bool
	logAutoscroll bool
}

func initialModel(targets map[string]*FilesystemTarget, statusMap map[string]*TargetStatus) *model {
	return &model{
		viewport:      viewport.New(160, 40),
		targets:       targets,
		statusMap:     statusMap,
		selectedIdx:   0,
		logView:       viewport.New(160, 20),
		logAutoscroll: true,
	}
}

func (m *model) Init() tea.Cmd {
	return tickCmd()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if !m.showingLogs {
				m.selectedIdx = (m.selectedIdx - 1 + len(m.targets)) % len(m.targets)
			} else {
				m.logAutoscroll = false
				m.logView, cmd = m.logView.Update(msg)
				cmds = append(cmds, cmd)
			}
		case "down", "j":
			if !m.showingLogs {
				m.selectedIdx = (m.selectedIdx + 1) % len(m.targets)
			} else {
				m.logView, cmd = m.logView.Update(msg)
				cmds = append(cmds, cmd)
			}
		case "enter", " ":
			m.showingLogs = !m.showingLogs
			if m.showingLogs {
				m.logAutoscroll = true
				m.updateLogView()
			} else {
				m.viewport = viewport.New(160, 40)
				m.viewport.SetContent(m.statusView())
			}
		case "esc":
			m.showingLogs = false
			m.viewport = viewport.New(160, 40)
			m.viewport.SetContent(m.statusView())
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 1
		m.logView.Width = msg.Width
		m.logView.Height = msg.Height / 2
		return m, nil
	case tickMsg:
		if !m.done {
			cmds = append(cmds, tickCmd())
		}
	}

	if !m.showingLogs {
		m.viewport = viewport.New(160, 40)
		m.viewport.SetContent(m.statusView())
	} else if m.logAutoscroll {
		m.updateLogView()
	}
	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.done {
		return "Exiting...\n"
	}
	var sb strings.Builder
	sb.WriteString(m.viewport.View())
	if m.showingLogs {
		sb.WriteString("\n\nOutput:\n")
		sb.WriteString(m.logView.View())
	}
	sb.WriteString("\n\033[1mPress q to quit, enter/space to toggle logs, up/down or j/k to navigate\033[0m")
	return sb.String()
}

func (m *model) statusView() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("Generooni Status Report\n\n")

	var targetNames []string
	for name := range m.targets {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	for i, name := range targetNames {
		status := m.statusMap[name]
		if status == nil {
			continue
		}

		var duration time.Duration
		if !status.EndTime.IsZero() {
			duration = status.EndTime.Sub(status.StartTime)
		} else if !status.StartTime.IsZero() {
			duration = time.Since(status.StartTime)
		}

		progress := ""
		if status.Status == "Running" {
			progress = strings.Repeat("=", status.FileChanges) + ">"
		}

		statusColor := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
		switch status.Status {
		case "Completed":
			statusColor = statusColor.Foreground(lipgloss.Color("82"))
		case "Failed":
			statusColor = statusColor.Foreground(lipgloss.Color("160"))
		case "Skipped":
			statusColor = statusColor.Foreground(lipgloss.Color("243"))
		}

		prefix := "  "
		if i == m.selectedIdx {
			prefix = "> "
		}

		sb.WriteString(fmt.Sprintf(
			"%s%-20s | %-10s | %-20s | %-10s | Changes: %d\n",
			prefix,
			name,
			statusColor.Render(status.Status),
			progress,
			duration.Round(time.Millisecond),
			status.FileChanges,
		))
	}

	return sb.String()
}

func (m *model) updateLogView() {
	targetNames := make([]string, 0, len(m.targets))
	for name := range m.targets {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	m.logView = viewport.New(160, 20)

	if m.selectedIdx < len(targetNames) {
		selectedTarget := targetNames[m.selectedIdx]
		status := m.statusMap[selectedTarget]
		if status != nil {
			logContent := strings.Join(status.LogLines, "\n")
			if logContent == "" {
				m.logView.SetContent("This job has not yet started")
			} else {
				m.logView.SetContent(logContent)
			}
			if m.logAutoscroll {
				m.logView.GotoBottom()
			}
		}
	}
}

func (te *TargetExecutor) reportStatus(done <-chan struct{}) {
	p := tea.NewProgram(initialModel(te.Targets, te.StatusMap))
	go func() {
		<-done
		p.Quit()
		//p.Kill()
	}()
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (te *TargetExecutor) getWaitingForTargets(name string) []string {
	var waitingFor []string
	for _, dep := range te.Targets[name].TargetDeps {
		depStatus := te.StatusMap[dep]
		if depStatus.Status != "Completed" && !depStatus.Skipped {
			waitingFor = append(waitingFor, dep)
		}
	}
	return waitingFor
}

func (te *TargetExecutor) updateChecksums(target *FilesystemTarget) error {
	inputHash, err := te.calculateInputChecksum(target)
	if err != nil {
		return err
	}
	target.InputHash = inputHash

	outputHash, err := te.calculateOutputChecksum(target)
	if err != nil {
		return err
	}
	target.OutputHash = outputHash

	return nil
}

func (te *TargetExecutor) calculateInputChecksum(target *FilesystemTarget) (string, error) {
	h := md5.New()

	for _, dep := range target.Dependencies {
		matches, err := filepath.Glob(dep)
		if err != nil {
			return "", err
		}
		for _, match := range matches {
			if err := hashPathRecursively(h, match); err != nil {
				return "", err
			}
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (te *TargetExecutor) calculateOutputChecksum(target *FilesystemTarget) (string, error) {
	h := md5.New()

	for _, output := range target.Outputs {
		matches, err := filepath.Glob(output)
		if err != nil {
			return "", err
		}
		for _, match := range matches {
			if err := hashFile(h, match); err != nil {
				return "", err
			}
		}
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

func (te *TargetExecutor) updateLogLines(name string, line string) {
	te.mu.Lock()
	defer te.mu.Unlock()

	status, exists := te.StatusMap[name]
	if !exists {
		// If the status doesn't exist, create it
		status = &TargetStatus{
			ExecutionStatus: &ExecutionStatus{
				Status: "Running", // Assume it's running if we're updating log lines
			},
			LogLines: make([]string, 0),
		}
		te.StatusMap[name] = status
	}

	status.LogLines = append(status.LogLines, line)
	if len(status.LogLines) > 100 {
		status.LogLines = status.LogLines[len(status.LogLines)-100:]
	}
}

func (te *TargetExecutor) readAndUpdateLogs(name string, pipe io.Reader) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		te.updateLogLines(name, scanner.Text())
	}
}

func main() {
	betaUI := flag.Bool("beta-ui", false, "Log all job command output to stdout")
	flag.Parse()

	executor := NewTargetExecutor()

	if err := executor.loadLockFile(); err != nil {
		log.Fatalf("Error loading lock file: %v", err)
	}

	targets, err := parseStarlarkConfig("generooni.star")
	if err != nil {
		log.Fatalf("Error parsing Starlark config: %v", err)
	}

	if *betaUI {
		defer func() {
			// Restore the terminal
			fmt.Print("\033[?1049l")
			fmt.Print("\033[?25h")
		}()
	}

	for _, target := range targets {
		executor.AddTarget(target)
	}

	if err := executor.ExecuteTargets(!*betaUI); err != nil {
		log.Fatalf("Error executing targets: %v", err)
	}
}
