package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ProcessStatus string

const (
	StatusStopped  ProcessStatus = "stopped"
	StatusStarting ProcessStatus = "starting"
	StatusRunning  ProcessStatus = "running"
	StatusError    ProcessStatus = "error"
)

type Status struct {
	Backend        ProcessStatus `json:"backend"`
	Frontend       ProcessStatus `json:"frontend"`
	BackendPID     int           `json:"backendPid"`
	FrontendPID    int           `json:"frontendPid"`
	MToolsExists   bool          `json:"mtoolsExists"`
	LibraryPath    string        `json:"libraryPath"`
	FrontendPort   string        `json:"frontendPort"`
	BackendPort    string        `json:"backendPort"`
	IsProduction   bool          `json:"isProduction"`
}

type Config struct {
	LibraryPath   string `json:"libraryPath"`
	FrontendPort  string `json:"frontendPort"`
	AutoLoadBooks bool   `json:"autoLoadBooks"`
	Language      string `json:"language"`
}

// WatcherStatus represents the backend watcher status
type WatcherStatus struct {
	Available bool              `json:"available"`
	Enabled   bool              `json:"enabled"`
	Files     map[string]string `json:"files,omitempty"`
}

// PortStatus represents information about a port
type PortStatus struct {
	Port   string `json:"port"`
	InUse  bool   `json:"inUse"`
	PID    int    `json:"pid,omitempty"`
	Killed bool   `json:"killed,omitempty"`
}

type App struct {
	ctx          context.Context
	mu           sync.Mutex
	backendCmd   *exec.Cmd
	frontendCmd  *exec.Cmd
	config       Config
	projectRoot  string
	backendLogs  []string
	frontendLogs []string
	logMu        sync.Mutex

	// Production mode
	dataDir          string       // Directory for extracted files
	backendPath      string       // Path to backend binary
	frontendDir      string       // Path to frontend static files
	frontendServer   *http.Server // Static file server for frontend
	frontendListener net.Listener
}

const (
	DefaultBackendPort  = "7754"
	DefaultFrontendPort = "7750"
	MaxLogEntries       = 200 // Maximum log entries to keep in memory per source
)

func NewApp() *App {
	return &App{
		config: Config{
			FrontendPort:  DefaultFrontendPort,
			AutoLoadBooks: false, // Default: don't auto-load books to prevent high CPU usage
		},
		backendLogs:  make([]string, 0, MaxLogEntries),
		frontendLogs: make([]string, 0, MaxLogEntries),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	if isProduction {
		a.setupProduction()
	} else {
		a.projectRoot = a.findProjectRoot()
		a.loadConfig()
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.StopAll()
	// Cleanup extracted files in production
	if isProduction && a.dataDir != "" {
		os.RemoveAll(a.dataDir)
	}
}

// setupProduction extracts bundled assets and prepares for production mode
func (a *App) setupProduction() {
	// Create data directory in user's app data folder
	dataDir, err := a.getAppDataDir()
	if err != nil {
		return
	}
	a.dataDir = dataDir

	// Extract backend binary
	backendName := "mtools-backend"
	if runtime.GOOS == "windows" {
		backendName += ".exe"
	}
	a.backendPath = filepath.Join(a.dataDir, backendName)

	if err := a.extractBackend(); err != nil {
		return
	}

	// Extract frontend files
	a.frontendDir = filepath.Join(a.dataDir, "frontend")
	if err := a.extractFrontend(); err != nil {
		return
	}
}

// getAppDataDir returns the application data directory
func (a *App) getAppDataDir() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		baseDir = filepath.Join(homeDir, "Library", "Application Support", "MTools")
	case "windows":
		baseDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "MTools")
	default: // Linux and others
		homeDir, _ := os.UserHomeDir()
		baseDir = filepath.Join(homeDir, ".local", "share", "mtools")
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}

	return baseDir, nil
}

// extractBackend extracts the embedded backend binary
func (a *App) extractBackend() error {
	if len(embeddedBackend) == 0 {
		return fmt.Errorf("no embedded backend binary")
	}

	// Check if already extracted and up to date
	if info, err := os.Stat(a.backendPath); err == nil {
		if info.Size() == int64(len(embeddedBackend)) {
			return nil // Already extracted
		}
	}

	// Write binary
	if err := os.WriteFile(a.backendPath, embeddedBackend, 0755); err != nil {
		return fmt.Errorf("failed to write backend binary: %w", err)
	}

	return nil
}

// extractFrontend extracts the embedded frontend files
func (a *App) extractFrontend() error {
	// Check if embeddedFrontend has any files
	entries, err := fs.ReadDir(embeddedFrontend, "bundled/frontend")
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no embedded frontend files")
	}

	// Remove old frontend dir
	os.RemoveAll(a.frontendDir)

	// Extract all files
	return fs.WalkDir(embeddedFrontend, "bundled/frontend", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, _ := filepath.Rel("bundled/frontend", path)
		destPath := filepath.Join(a.frontendDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Read and write file
		data, err := fs.ReadFile(embeddedFrontend, path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

func (a *App) findProjectRoot() string {
	// Get executable path
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		// Check if we're in launcher directory
		if filepath.Base(dir) == "launcher" {
			return filepath.Dir(dir)
		}
		// Check parent directories for frontend/backend folders
		for i := 0; i < 5; i++ {
			if a.isValidProjectRoot(dir) {
				return dir
			}
			dir = filepath.Dir(dir)
		}
	}

	// Fallback: try current working directory
	cwd, _ := os.Getwd()
	if filepath.Base(cwd) == "launcher" {
		return filepath.Dir(cwd)
	}
	if a.isValidProjectRoot(cwd) {
		return cwd
	}

	return cwd
}

func (a *App) isValidProjectRoot(dir string) bool {
	_, errFE := os.Stat(filepath.Join(dir, "frontend"))
	_, errBE := os.Stat(filepath.Join(dir, "backend"))
	return errFE == nil && errBE == nil
}

func (a *App) loadConfig() {
	// TODO: Enable config persistence when needed
	// // Try to load saved config first
	// configPath := a.getConfigPath()
	// if data, err := os.ReadFile(configPath); err == nil {
	// 	var savedConfig Config
	// 	if err := json.Unmarshal(data, &savedConfig); err == nil {
	// 		a.config = savedConfig
	// 		// Ensure defaults for new fields
	// 		if a.config.FrontendPort == "" {
	// 			a.config.FrontendPort = DefaultFrontendPort
	// 		}
	// 		return
	// 	}
	// }

	// Fallback: Try to find default library in testdata
	defaultLibrary := filepath.Join(a.projectRoot, "backend", "testdata", "library")
	if _, err := os.Stat(defaultLibrary); err == nil {
		a.config.LibraryPath = defaultLibrary
	}
}

func (a *App) getConfigPath() string {
	if isProduction {
		return filepath.Join(a.dataDir, "config.json")
	}
	return filepath.Join(a.projectRoot, "launcher", "config.json")
}

func (a *App) saveConfigToFile() error {
	// TODO: Enable config persistence when needed
	return nil

	// configPath := a.getConfigPath()
	//
	// // Ensure directory exists
	// dir := filepath.Dir(configPath)
	// if err := os.MkdirAll(dir, 0755); err != nil {
	// 	return err
	// }
	//
	// data, err := json.MarshalIndent(a.config, "", "  ")
	// if err != nil {
	// 	return err
	// }
	//
	// return os.WriteFile(configPath, data, 0644)
}

// GetStatus returns current status of all processes
func (a *App) GetStatus() Status {
	a.mu.Lock()
	defer a.mu.Unlock()

	status := Status{
		Backend:      StatusStopped,
		Frontend:     StatusStopped,
		LibraryPath:  a.config.LibraryPath,
		FrontendPort: a.config.FrontendPort,
		BackendPort:  DefaultBackendPort,
		IsProduction: isProduction,
	}

	if isProduction {
		status.MToolsExists = a.backendPath != "" && a.frontendDir != ""
	} else {
		status.MToolsExists = a.isValidProjectRoot(a.projectRoot)
	}

	if a.backendCmd != nil && a.backendCmd.Process != nil {
		status.Backend = StatusRunning
		status.BackendPID = a.backendCmd.Process.Pid
	}

	if isProduction {
		if a.frontendServer != nil {
			status.Frontend = StatusRunning
		}
	} else {
		if a.frontendCmd != nil && a.frontendCmd.Process != nil {
			status.Frontend = StatusRunning
			status.FrontendPID = a.frontendCmd.Process.Pid
		}
	}

	return status
}

// GetConfig returns current configuration
func (a *App) GetConfig() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config
}

// GetLanguage returns the current UI language
func (a *App) GetLanguage() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.config.Language == "" {
		return "en"
	}
	return a.config.Language
}

// SetLanguage sets the UI language and saves config
func (a *App) SetLanguage(lang string) error {
	// Supported languages
	supported := map[string]bool{
		"en": true, "ru": true, "es": true, "zh": true, "fr": true,
		"it": true, "de": true, "ko": true, "pt": true, "el": true,
		"tr": true, "vi": true, "th": true, "fi": true,
	}
	if !supported[lang] {
		return fmt.Errorf("unsupported language: %s", lang)
	}
	a.mu.Lock()
	a.config.Language = lang
	a.mu.Unlock()
	return a.saveConfigToFile()
}

// SetConfig updates configuration and saves to file
func (a *App) SetConfig(config Config) error {
	a.mu.Lock()
	a.config = config
	a.mu.Unlock()

	// Persist to file
	return a.saveConfigToFile()
}

// SelectLibraryFolder opens native folder picker
func (a *App) SelectLibraryFolder() (string, error) {
	// Get current home directory as default
	homeDir, _ := os.UserHomeDir()
	defaultDir := homeDir
	if a.config.LibraryPath != "" {
		defaultDir = a.config.LibraryPath
	}

	// Bring window to front to ensure dialog appears on top
	wailsRuntime.WindowShow(a.ctx)

	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title:            "Select Library Folder",
		DefaultDirectory: defaultDir,
	})
	if err != nil {
		return "", err
	}

	if path != "" {
		a.mu.Lock()
		a.config.LibraryPath = path
		a.mu.Unlock()
	}
	return path, nil
}

// StartBackend starts the Go backend
func (a *App) StartBackend() error {
	a.mu.Lock()
	if a.backendCmd != nil && a.backendCmd.Process != nil {
		a.mu.Unlock()
		return fmt.Errorf("backend is already running")
	}

	if a.config.LibraryPath == "" {
		a.mu.Unlock()
		return fmt.Errorf("library path is not set")
	}

	var cmd *exec.Cmd

	// Build args list
	args := []string{"-library", a.config.LibraryPath}
	if a.config.AutoLoadBooks {
		args = append(args, "-autoload-books")
	}

	if isProduction {
		// Production: use extracted binary
		cmd = exec.Command(a.backendPath, args...)
		cmd.Dir = a.dataDir // Ensure certs are created in app data dir
		a.emitLog("backend", "Starting backend (production mode)...")
	} else {
		// Development: use go run
		backendDir := filepath.Join(a.projectRoot, "backend")
		goArgs := append([]string{"run", "./cmd"}, args...)
		cmd = exec.Command("go", goArgs...)
		cmd.Dir = backendDir
		a.emitLog("backend", "Starting backend (development mode)...")
	}

	if !a.config.AutoLoadBooks {
		a.emitLog("backend", "Auto-load books disabled (use frontend to start loading)")
	}

	setupProcessGroup(cmd)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		a.mu.Unlock()
		return fmt.Errorf("failed to start backend: %w", err)
	}

	a.backendCmd = cmd
	a.mu.Unlock()

	a.emitLog("backend", fmt.Sprintf("Backend started (PID: %d)", cmd.Process.Pid))

	go a.streamOutput(stdout, "backend")
	go a.streamOutput(stderr, "backend")
	go a.waitForProcess(cmd, "backend")

	return nil
}

// StartFrontend starts the Svelte frontend
func (a *App) StartFrontend() error {
	a.mu.Lock()

	if isProduction {
		// Production: start static file server
		if a.frontendServer != nil {
			a.mu.Unlock()
			return fmt.Errorf("frontend is already running")
		}

		a.emitLog("frontend", "Starting frontend (production mode)...")

		// Create server with SPA fallback
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Clean URL path
			urlPath := r.URL.Path
			if urlPath == "/" {
				urlPath = "/index.html"
			}

			// Try to serve the file
			filePath := filepath.Join(a.frontendDir, filepath.Clean(urlPath))

			// Check if file exists
			info, err := os.Stat(filePath)
			if err == nil && !info.IsDir() {
				// File exists, serve it with proper content type
				http.ServeFile(w, r, filePath)
				return
			}

			// Check if it's a directory with index.html
			if err == nil && info.IsDir() {
				indexPath := filepath.Join(filePath, "index.html")
				if _, err := os.Stat(indexPath); err == nil {
					http.ServeFile(w, r, indexPath)
					return
				}
			}

			// SPA fallback: only for routes (no extension = likely a route)
			ext := filepath.Ext(urlPath)
			if ext == "" || ext == ".html" {
				http.ServeFile(w, r, filepath.Join(a.frontendDir, "index.html"))
				return
			}

			// Asset not found - return 404
			http.NotFound(w, r)
		})

		listener, err := net.Listen("tcp", ":"+a.config.FrontendPort)
		if err != nil {
			a.mu.Unlock()
			return fmt.Errorf("failed to listen on port %s: %w", a.config.FrontendPort, err)
		}

		a.frontendListener = listener
		a.frontendServer = &http.Server{Handler: mux}
		a.mu.Unlock()

		a.emitLog("frontend", fmt.Sprintf("Frontend serving at http://localhost:%s", a.config.FrontendPort))

		// Start server in goroutine
		go func() {
			if err := a.frontendServer.Serve(listener); err != nil && err != http.ErrServerClosed {
				a.emitLog("frontend", fmt.Sprintf("Frontend server error: %v", err))
			}
			a.emitLog("frontend", "Frontend stopped")
			wailsRuntime.EventsEmit(a.ctx, "statusChange", a.GetStatus())
		}()

		return nil
	}

	// Development mode
	if a.frontendCmd != nil && a.frontendCmd.Process != nil {
		a.mu.Unlock()
		return fmt.Errorf("frontend is already running")
	}

	frontendDir := filepath.Join(a.projectRoot, "frontend")

	// Check for node_modules
	nodeModules := filepath.Join(frontendDir, "node_modules")
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		a.mu.Unlock()
		a.emitLog("frontend", "Installing dependencies...")

		installCmd := exec.Command("pnpm", "install")
		installCmd.Dir = frontendDir

		output, err := installCmd.CombinedOutput()
		a.emitLog("frontend", string(output))

		if err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}
		a.emitLog("frontend", "Dependencies installed")
		a.mu.Lock()
	}

	cmd := exec.Command("pnpm", "run", "dev", "--port", a.config.FrontendPort)
	cmd.Dir = frontendDir
	setupProcessGroup(cmd)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		a.mu.Unlock()
		return fmt.Errorf("failed to start frontend: %w", err)
	}

	a.frontendCmd = cmd
	a.mu.Unlock()

	a.emitLog("frontend", fmt.Sprintf("Frontend started (PID: %d)", cmd.Process.Pid))

	go a.streamOutput(stdout, "frontend")
	go a.streamOutput(stderr, "frontend")
	go a.waitForProcess(cmd, "frontend")

	return nil
}

// StartAll starts both backend and frontend
func (a *App) StartAll() error {
	if err := a.StartBackend(); err != nil {
		return err
	}
	// Small delay to let backend start first
	time.Sleep(500 * time.Millisecond)
	return a.StartFrontend()
}

// StopBackend stops the backend process
func (a *App) StopBackend() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.backendCmd == nil || a.backendCmd.Process == nil {
		return nil
	}

	a.emitLog("backend", "Stopping backend...")
	killProcessGroup(a.backendCmd.Process.Pid)
	a.backendCmd = nil
	a.emitLog("backend", "Backend stopped")
	return nil
}

// StopFrontend stops the frontend process/server
func (a *App) StopFrontend() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if isProduction {
		if a.frontendServer == nil {
			return nil
		}
		a.emitLog("frontend", "Stopping frontend...")
		a.frontendServer.Close()
		if a.frontendListener != nil {
			a.frontendListener.Close()
		}
		a.frontendServer = nil
		a.frontendListener = nil
		a.emitLog("frontend", "Frontend stopped")
		return nil
	}

	if a.frontendCmd == nil || a.frontendCmd.Process == nil {
		return nil
	}

	a.emitLog("frontend", "Stopping frontend...")
	killProcessGroup(a.frontendCmd.Process.Pid)
	a.frontendCmd = nil
	a.emitLog("frontend", "Frontend stopped")
	return nil
}

// StopAll stops all processes
func (a *App) StopAll() error {
	a.StopBackend()
	a.StopFrontend()
	return nil
}

// RestartAll restarts all processes
func (a *App) RestartAll() error {
	a.StopAll()
	time.Sleep(500 * time.Millisecond)
	return a.StartAll()
}

// GetLogs returns recent logs
func (a *App) GetLogs(source string, limit int) []string {
	a.logMu.Lock()
	defer a.logMu.Unlock()

	var logs []string
	switch source {
	case "backend":
		logs = a.backendLogs
	case "frontend":
		logs = a.frontendLogs
	default:
		// Merge both logs (simplified - just concat)
		logs = append(a.backendLogs, a.frontendLogs...)
	}

	if limit > 0 && len(logs) > limit {
		return logs[len(logs)-limit:]
	}
	return logs
}

// ClearLogs clears logs
func (a *App) ClearLogs(source string) {
	a.logMu.Lock()
	defer a.logMu.Unlock()

	switch source {
	case "backend":
		a.backendLogs = a.backendLogs[:0]
	case "frontend":
		a.frontendLogs = a.frontendLogs[:0]
	default:
		a.backendLogs = a.backendLogs[:0]
		a.frontendLogs = a.frontendLogs[:0]
	}
}

// OpenMTools opens the mtools frontend in default browser
func (a *App) OpenMTools() error {
	lang := a.GetLanguage()
	url := fmt.Sprintf("http://localhost:%s/?lang=%s", a.config.FrontendPort, lang)
	return openURL(url)
}

// OpenMToolsAPI opens the backend API in default browser
func (a *App) OpenMToolsAPI() error {
	url := fmt.Sprintf("http://localhost:%s", DefaultBackendPort)
	return openURL(url)
}

// GetFrontendURL returns the frontend URL for embedding (with language parameter)
func (a *App) GetFrontendURL() string {
	lang := a.GetLanguage()
	return fmt.Sprintf("http://localhost:%s/?lang=%s", a.config.FrontendPort, lang)
}

// CheckDependencies checks for required dependencies
func (a *App) CheckDependencies() map[string]bool {
	deps := map[string]bool{
		"go":   false,
		"node": false,
		"pnpm": false,
	}

	// In production mode, no external dependencies needed
	if isProduction {
		deps["go"] = true
		deps["node"] = true
		deps["pnpm"] = true
		return deps
	}

	if _, err := exec.LookPath("go"); err == nil {
		deps["go"] = true
	}
	if _, err := exec.LookPath("node"); err == nil {
		deps["node"] = true
	}
	if _, err := exec.LookPath("pnpm"); err == nil {
		deps["pnpm"] = true
	}

	return deps
}

// IsProductionMode returns whether the app is running in production mode
func (a *App) IsProductionMode() bool {
	return isProduction
}

func (a *App) streamOutput(pipe io.ReadCloser, source string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		a.emitLog(source, line)
	}
}

func (a *App) emitLog(source, message string) {
	a.logMu.Lock()
	timestamp := time.Now().Format("15:04:05")
	logLine := fmt.Sprintf("[%s] %s", timestamp, message)

	switch source {
	case "backend":
		a.backendLogs = append(a.backendLogs, logLine)
		if len(a.backendLogs) > MaxLogEntries {
			a.backendLogs = a.backendLogs[len(a.backendLogs)-MaxLogEntries:]
		}
	case "frontend":
		a.frontendLogs = append(a.frontendLogs, logLine)
		if len(a.frontendLogs) > MaxLogEntries {
			a.frontendLogs = a.frontendLogs[len(a.frontendLogs)-MaxLogEntries:]
		}
	}
	a.logMu.Unlock()

	// Emit event to frontend
	wailsRuntime.EventsEmit(a.ctx, "log", map[string]string{
		"source":  source,
		"message": logLine,
	})
}

func (a *App) waitForProcess(cmd *exec.Cmd, name string) {
	cmd.Wait()
	a.mu.Lock()
	switch name {
	case "backend":
		a.backendCmd = nil
	case "frontend":
		a.frontendCmd = nil
	}
	a.mu.Unlock()
	a.emitLog(name, fmt.Sprintf("%s process exited", name))

	// Emit status change
	wailsRuntime.EventsEmit(a.ctx, "statusChange", a.GetStatus())
}

// CheckPortInUse checks if a port is in use and returns the PID if possible
func (a *App) CheckPortInUse(port string) PortStatus {
	status := PortStatus{Port: port, InUse: false}

	// Try to listen on the port to check if it's in use
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		status.InUse = true
		// Try to get PID using lsof (macOS/Linux)
		if runtime.GOOS != "windows" {
			cmd := exec.Command("lsof", "-ti", ":"+port)
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				var pid int
				fmt.Sscanf(string(output), "%d", &pid)
				status.PID = pid
			}
		}
	} else {
		listener.Close()
	}

	return status
}

// KillProcessOnPort kills any process using the specified port
func (a *App) KillProcessOnPort(port string) PortStatus {
	status := a.CheckPortInUse(port)
	if !status.InUse {
		return status
	}

	a.emitLog("backend", fmt.Sprintf("Killing process on port %s...", port))

	if runtime.GOOS == "windows" {
		// Windows: use netstat to find PID and taskkill
		cmd := exec.Command("cmd", "/c", fmt.Sprintf("for /f \"tokens=5\" %%a in ('netstat -aon ^| findstr :%s') do taskkill /F /PID %%a", port))
		cmd.Run()
	} else {
		// macOS/Linux: use lsof and kill
		cmd := exec.Command("sh", "-c", fmt.Sprintf("lsof -ti :%s | xargs kill -9 2>/dev/null", port))
		cmd.Run()
	}

	// Wait a bit and check again
	time.Sleep(500 * time.Millisecond)
	newStatus := a.CheckPortInUse(port)
	newStatus.Killed = !newStatus.InUse
	return newStatus
}

// GetPortsStatus returns the status of both backend and frontend ports
func (a *App) GetPortsStatus() []PortStatus {
	return []PortStatus{
		a.CheckPortInUse(DefaultBackendPort),
		a.CheckPortInUse(a.config.FrontendPort),
	}
}

// KillPortProcesses kills processes on both backend and frontend ports
func (a *App) KillPortProcesses() []PortStatus {
	return []PortStatus{
		a.KillProcessOnPort(DefaultBackendPort),
		a.KillProcessOnPort(a.config.FrontendPort),
	}
}

// GetWatcherStatus gets the watcher status from the backend API
func (a *App) GetWatcherStatus() (WatcherStatus, error) {
	status := WatcherStatus{Available: false, Enabled: false}

	// Check if backend is running
	if a.backendCmd == nil || a.backendCmd.Process == nil {
		return status, nil
	}

	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/api/watcher/status", DefaultBackendPort))
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()

	var result struct {
		Data WatcherStatus `json:"data"`
	}
	if err := parseJSONResponse(resp, &result); err != nil {
		return status, err
	}

	return result.Data, nil
}

// SetWatcherEnabled enables or disables the watcher via backend API
func (a *App) SetWatcherEnabled(enabled bool) error {
	// Check if backend is running
	if a.backendCmd == nil || a.backendCmd.Process == nil {
		return fmt.Errorf("backend is not running")
	}

	url := fmt.Sprintf("http://localhost:%s/api/watcher/enable", DefaultBackendPort)
	var req *http.Request
	var err error

	if enabled {
		req, err = http.NewRequest("POST", url, nil)
	} else {
		req, err = http.NewRequest("DELETE", url, nil)
	}
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set watcher: status %d", resp.StatusCode)
	}

	return nil
}

// parseJSONResponse helper to parse JSON response
func parseJSONResponse(resp *http.Response, v interface{}) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, v)
}

func openURL(urlStr string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	default:
		cmd = exec.Command("xdg-open", urlStr)
	}
	return cmd.Start()
}
