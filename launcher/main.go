package main

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"
)

// Default ports
const (
	DefaultFrontendPort = 7750
)

type DependencyStatus struct {
	Name      string
	Installed bool
	Version   string
	URL       string
}

type ProcessManager struct {
	mu          sync.Mutex
	backendCmd  *exec.Cmd
	frontendCmd *exec.Cmd
}

func (pm *ProcessManager) StopAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.backendCmd != nil && pm.backendCmd.Process != nil {
		// Kill the entire process group (negative PID)
		killProcessGroup(pm.backendCmd.Process.Pid)
		pm.backendCmd = nil
	}
	if pm.frontendCmd != nil && pm.frontendCmd.Process != nil {
		// Kill the entire process group (negative PID)
		killProcessGroup(pm.frontendCmd.Process.Pid)
		pm.frontendCmd = nil
	}
}

// killProcessGroup kills a process and all its children
func killProcessGroup(pid int) {
	// On Unix, kill the process group by using negative PID
	if runtime.GOOS != "windows" {
		syscall.Kill(-pid, syscall.SIGKILL)
	} else {
		// On Windows, use taskkill /T to kill tree
		exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(pid)).Run()
	}
}

// killProcessByPort kills any process listening on the specified port
func killProcessByPort(port string) error {
	if runtime.GOOS == "windows" {
		// Windows: find PID using netstat and kill
		cmd := exec.Command("cmd", "/c", fmt.Sprintf("for /f \"tokens=5\" %%a in ('netstat -aon ^| findstr :%s') do taskkill /F /PID %%a", port))
		return cmd.Run()
	}
	// macOS/Linux: use lsof to find and kill
	cmd := exec.Command("sh", "-c", fmt.Sprintf("lsof -ti:%s | xargs kill -9 2>/dev/null", port))
	return cmd.Run()
}

// setupProcessGroup configures cmd to run in its own process group
func setupProcessGroup(cmd *exec.Cmd) {
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}

func checkCommand(name string, args ...string) (bool, string) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}
	version := strings.TrimSpace(string(output))
	// Extract first line only
	if idx := strings.Index(version, "\n"); idx != -1 {
		version = version[:idx]
	}
	return true, version
}

func checkDependencies() []DependencyStatus {
	deps := []DependencyStatus{}

	// Check Go
	installed, version := checkCommand("go", "version")
	deps = append(deps, DependencyStatus{
		Name:      "Go",
		Installed: installed,
		Version:   version,
		URL:       "https://go.dev/dl/",
	})

	// Check pnpm (preferred)
	installed, version = checkCommand("pnpm", "--version")
	deps = append(deps, DependencyStatus{
		Name:      "pnpm",
		Installed: installed,
		Version:   version,
		URL:       "https://pnpm.io/installation",
	})

	// Check npm (fallback)
	installed, version = checkCommand("npm", "--version")
	deps = append(deps, DependencyStatus{
		Name:      "npm",
		Installed: installed,
		Version:   version,
		URL:       "https://nodejs.org/",
	})

	// Check Node.js
	installed, version = checkCommand("node", "--version")
	deps = append(deps, DependencyStatus{
		Name:      "Node.js",
		Installed: installed,
		Version:   version,
		URL:       "https://nodejs.org/",
	})

	return deps
}

func getProjectRoot() string {
	// Try to find the project root relative to executable
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		// Check if we're in launcher directory
		if filepath.Base(dir) == "launcher" {
			return filepath.Dir(dir)
		}
		// Check parent directories
		for i := 0; i < 3; i++ {
			if _, err := os.Stat(filepath.Join(dir, "frontend")); err == nil {
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
	return cwd
}

func main() {
	projectRoot := getProjectRoot()
	pm := &ProcessManager{}

	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Mnemoo Tools | Launcher")
	w.Resize(fyne.NewSize(700, 700))

	// Status labels
	backendStatus := widget.NewLabel("Backend: Stopped")
	backendStatus.TextStyle = fyne.TextStyle{Bold: true}
	frontendStatus := widget.NewLabel("Frontend: Stopped")
	frontendStatus.TextStyle = fyne.TextStyle{Bold: true}

	// Log output
	logOutput := widget.NewMultiLineEntry()
	logOutput.Wrapping = fyne.TextWrapWord
	logOutput.TextStyle = fyne.TextStyle{Monospace: true}

	appendLog := func(msg string) {
		logOutput.SetText(logOutput.Text + msg + "\n")
		// Auto-scroll to bottom
		logOutput.CursorRow = len(strings.Split(logOutput.Text, "\n")) - 1
	}

	// Dependency checks
	deps := checkDependencies()
	depItems := []fyne.CanvasObject{}

	hasPnpm := false
	hasNpm := false
	hasGo := false
	hasNode := false

	for _, dep := range deps {
		statusIcon := widget.NewIcon(theme.ConfirmIcon())
		if !dep.Installed {
			statusIcon = widget.NewIcon(theme.CancelIcon())
		}

		var versionLabel *widget.Label
		if dep.Installed {
			versionLabel = widget.NewLabel(dep.Version)
		} else {
			versionLabel = widget.NewLabel("Not installed")
		}

		urlLink := widget.NewHyperlink("Install", parseURL(dep.URL))

		row := container.NewHBox(
			statusIcon,
			widget.NewLabel(dep.Name+":"),
			versionLabel,
		)
		if !dep.Installed {
			row.Add(urlLink)
		}

		depItems = append(depItems, row)

		switch dep.Name {
		case "pnpm":
			hasPnpm = dep.Installed
		case "npm":
			hasNpm = dep.Installed
		case "Go":
			hasGo = dep.Installed
		case "Node.js":
			hasNode = dep.Installed
		}
	}

	canRunFrontend := (hasPnpm || hasNpm) && hasNode
	canRunBackend := hasGo

	// Index file path entry
	indexPathEntry := widget.NewEntry()
	indexPathEntry.SetPlaceHolder("Path to index.json (required for backend)")

	// Try to find default index.json in testdata
	defaultIndex := filepath.Join(projectRoot, "backend", "testdata", "index.json")
	if _, err := os.Stat(defaultIndex); err == nil {
		indexPathEntry.SetText(defaultIndex)
	}

	browseBtn := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		// Use native OS file picker via zenity
		file, err := zenity.SelectFile(
			zenity.Title("Select index.json"),
			zenity.FileFilters{
				{Name: "JSON files", Patterns: []string{"*.json"}, CaseFold: true},
				{Name: "All files", Patterns: []string{"*"}},
			},
		)
		if err == nil && file != "" {
			indexPathEntry.SetText(file)
		}
	})

	// Port configuration with numeric validation
	portValidator := func(s string) error {
		if s == "" {
			return fmt.Errorf("port required")
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				return fmt.Errorf("port must be a number")
			}
		}
		return nil
	}

	frontendPortEntry := widget.NewEntry()
	frontendPortEntry.SetText(fmt.Sprintf("%d", DefaultFrontendPort))
	frontendPortEntry.SetPlaceHolder("Frontend port")
	frontendPortEntry.Validator = portValidator

	// Kill port entry
	killPortEntry := widget.NewEntry()
	killPortEntry.SetPlaceHolder("Port to kill")

	// Process output streaming
	streamOutput := func(pipe io.ReadCloser, prefix string) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := scanner.Text()
			appendLog(fmt.Sprintf("[%s] %s", prefix, line))
		}
	}

	// Start buttons
	var startBackendBtn, startFrontendBtn, startAllBtn, stopAllBtn *widget.Button

	startBackend := func() {
		if !canRunBackend {
			appendLog("Error: Go is not installed")
			return
		}

		indexPath := indexPathEntry.Text
		if indexPath == "" {
			appendLog("Error: Index path is required")
			dialog.ShowError(fmt.Errorf("Please specify path to index.json"), w)
			return
		}

		pm.mu.Lock()
		if pm.backendCmd != nil {
			pm.mu.Unlock()
			appendLog("Backend is already running")
			return
		}

		backendDir := filepath.Join(projectRoot, "backend")
		cmd := exec.Command("go", "run", "./cmd",
			"-index", indexPath,
		)
		cmd.Dir = backendDir
		setupProcessGroup(cmd)

		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			pm.mu.Unlock()
			appendLog(fmt.Sprintf("Failed to start backend: %v", err))
			return
		}

		pm.backendCmd = cmd
		pm.mu.Unlock()

		backendStatus.SetText("Backend: Running (PID: " + fmt.Sprint(cmd.Process.Pid) + ")")
		appendLog("Backend started")

		go streamOutput(stdout, "BE")
		go streamOutput(stderr, "BE")
		go func() {
			cmd.Wait()
			pm.mu.Lock()
			pm.backendCmd = nil
			pm.mu.Unlock()
			backendStatus.SetText("Backend: Stopped")
			appendLog("Backend stopped")
		}()
	}

	startFrontend := func() {
		if !canRunFrontend {
			appendLog("Error: npm/pnpm or Node.js is not installed")
			return
		}

		pm.mu.Lock()
		if pm.frontendCmd != nil {
			pm.mu.Unlock()
			appendLog("Frontend is already running")
			return
		}
		pm.mu.Unlock()

		frontendDir := filepath.Join(projectRoot, "frontend")

		// Prefer pnpm over npm
		pkgManager := "npm"
		if hasPnpm {
			pkgManager = "pnpm"
		}

		// Check if node_modules exists, if not run install
		nodeModulesPath := filepath.Join(frontendDir, "node_modules")
		if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
			appendLog(fmt.Sprintf("Installing dependencies with %s...", pkgManager))
			frontendStatus.SetText("Frontend: Installing dependencies...")

			installCmd := exec.Command(pkgManager, "install")
			installCmd.Dir = frontendDir

			installStdout, _ := installCmd.StdoutPipe()
			installStderr, _ := installCmd.StderrPipe()

			if err := installCmd.Start(); err != nil {
				appendLog(fmt.Sprintf("Failed to install dependencies: %v", err))
				frontendStatus.SetText("Frontend: Install failed")
				return
			}

			go streamOutput(installStdout, "FE-INSTALL")
			go streamOutput(installStderr, "FE-INSTALL")

			if err := installCmd.Wait(); err != nil {
				appendLog(fmt.Sprintf("Dependency installation failed: %v", err))
				frontendStatus.SetText("Frontend: Install failed")
				return
			}
			appendLog("Dependencies installed successfully")
		}

		pm.mu.Lock()
		if pm.frontendCmd != nil {
			pm.mu.Unlock()
			appendLog("Frontend is already running")
			return
		}

		fePort := frontendPortEntry.Text
		cmd := exec.Command(pkgManager, "run", "dev", "--port", fePort)
		cmd.Dir = frontendDir
		setupProcessGroup(cmd)

		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			pm.mu.Unlock()
			appendLog(fmt.Sprintf("Failed to start frontend: %v", err))
			return
		}

		pm.frontendCmd = cmd
		pm.mu.Unlock()

		frontendStatus.SetText("Frontend: Running (PID: " + fmt.Sprint(cmd.Process.Pid) + ")")
		appendLog(fmt.Sprintf("Frontend started with %s", pkgManager))

		go streamOutput(stdout, "FE")
		go streamOutput(stderr, "FE")
		go func() {
			cmd.Wait()
			pm.mu.Lock()
			pm.frontendCmd = nil
			pm.mu.Unlock()
			frontendStatus.SetText("Frontend: Stopped")
			appendLog("Frontend stopped")
		}()
	}

	stopAll := func() {
		pm.StopAll()
		backendStatus.SetText("Backend: Stopped")
		frontendStatus.SetText("Frontend: Stopped")
		appendLog("All processes stopped")
	}

	startBackendBtn = widget.NewButtonWithIcon("Start Backend", theme.MediaPlayIcon(), startBackend)
	startFrontendBtn = widget.NewButtonWithIcon("Start Frontend", theme.MediaPlayIcon(), startFrontend)
	startAllBtn = widget.NewButtonWithIcon("Start All", theme.MediaPlayIcon(), func() {
		startBackend()
		startFrontend()
	})
	stopAllBtn = widget.NewButtonWithIcon("Stop All", theme.MediaStopIcon(), stopAll)
	stopAllBtn.Importance = widget.DangerImportance

	// Disable buttons if dependencies missing
	if !canRunBackend {
		startBackendBtn.Disable()
	}
	if !canRunFrontend {
		startFrontendBtn.Disable()
	}
	if !canRunBackend && !canRunFrontend {
		startAllBtn.Disable()
	}

	// Open URLs
	openFrontendBtn := widget.NewButtonWithIcon("Open Frontend", theme.ComputerIcon(), func() {
		openURL("http://localhost:" + frontendPortEntry.Text)
	})
	openBackendBtn := widget.NewButtonWithIcon("Open Backend API", theme.ComputerIcon(), func() {
		openURL("http://localhost:7754")
	})

	// Layout
	header := widget.NewLabelWithStyle("Mnemoo Tools | Launcher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	depsCard := widget.NewCard("Dependencies", "", container.NewVBox(depItems...))

	indexCard := widget.NewCard("Backend Configuration", "",
		container.NewBorder(nil, nil, nil, browseBtn, indexPathEntry),
	)

	// Port configuration card
	portsCard := widget.NewCard("Ports", "",
		container.NewVBox(
			container.NewGridWithColumns(2,
				widget.NewLabel("Frontend"),
				frontendPortEntry,
			),
		),
	)

	killByPortBtn := widget.NewButtonWithIcon("Kill Port", theme.DeleteIcon(), func() {
		port := killPortEntry.Text
		if port == "" {
			appendLog("Error: Enter port to kill")
			return
		}
		if err := killProcessByPort(port); err != nil {
			appendLog(fmt.Sprintf("Kill port %s: no process found or error", port))
		} else {
			appendLog(fmt.Sprintf("Killed process on port %s", port))
		}
	})
	killByPortBtn.Importance = widget.DangerImportance

	killBackendBtn := widget.NewButtonWithIcon("Kill Backend (7754)", theme.DeleteIcon(), func() {
		if err := killProcessByPort("7754"); err != nil {
			appendLog("Kill backend: no process found or error")
		} else {
			appendLog("Killed process on port 7754 (backend)")
		}
		pm.mu.Lock()
		pm.backendCmd = nil
		pm.mu.Unlock()
		backendStatus.SetText("Backend: Stopped")
	})
	killBackendBtn.Importance = widget.DangerImportance

	killFrontendBtn := widget.NewButtonWithIcon("Kill Frontend (7750)", theme.DeleteIcon(), func() {
		if err := killProcessByPort("7750"); err != nil {
			appendLog("Kill frontend: no process found or error")
		} else {
			appendLog("Killed process on port 7750 (frontend)")
		}
		pm.mu.Lock()
		pm.frontendCmd = nil
		pm.mu.Unlock()
		frontendStatus.SetText("Frontend: Stopped")
	})
	killFrontendBtn.Importance = widget.DangerImportance

	processKillerCard := widget.NewCard("Process Killer", "Kill processes by port",
		container.NewVBox(
			container.NewBorder(nil, nil, nil, killByPortBtn, killPortEntry),
			widget.NewSeparator(),
			container.NewGridWithColumns(2, killBackendBtn, killFrontendBtn),
		),
	)

	statusCard := widget.NewCard("Status", "",
		container.NewVBox(backendStatus, frontendStatus),
	)

	controlsCard := widget.NewCard("Controls", "",
		container.NewVBox(
			container.NewGridWithColumns(2, startBackendBtn, startFrontendBtn),
			container.NewGridWithColumns(2, startAllBtn, stopAllBtn),
			widget.NewSeparator(),
			container.NewGridWithColumns(2, openFrontendBtn, openBackendBtn),
		),
	)

	// Compact log preview (5 lines)
	logOutput.SetMinRowsVisible(5)
	logScroll := container.NewVScroll(logOutput)
	logScroll.SetMinSize(fyne.NewSize(0, 100))

	// Full log popup window
	viewFullLogBtn := widget.NewButtonWithIcon("View Full Log", theme.ListIcon(), func() {
		logWindow := a.NewWindow("Full Log")
		logWindow.Resize(fyne.NewSize(800, 600))

		fullLogText := widget.NewMultiLineEntry()
		fullLogText.SetText(logOutput.Text)
		fullLogText.Wrapping = fyne.TextWrapWord
		fullLogText.TextStyle = fyne.TextStyle{Monospace: true}

		clearBtn := widget.NewButtonWithIcon("Clear Log", theme.DeleteIcon(), func() {
			logOutput.SetText("")
			fullLogText.SetText("")
		})

		logWindow.SetContent(container.NewBorder(
			nil,
			container.NewHBox(clearBtn),
			nil, nil,
			container.NewVScroll(fullLogText),
		))
		logWindow.Show()
	})

	logCard := widget.NewCard("Logs", "",
		container.NewBorder(nil, viewFullLogBtn, nil, nil, logScroll),
	)

	// Main layout - everything in VBox with scroll
	content := container.NewVScroll(container.NewVBox(
		header,
		depsCard,
		indexCard,
		portsCard,
		statusCard,
		controlsCard,
		processKillerCard,
		logCard,
	))

	w.SetContent(content)

	// Cleanup on close
	w.SetOnClosed(func() {
		pm.StopAll()
	})

	w.ShowAndRun()
}

func parseURL(urlStr string) *url.URL {
	u, _ := url.Parse(urlStr)
	return u
}

func openURL(urlStr string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	default:
		cmd = exec.Command("xdg-open", urlStr)
	}

	// Start the command and wait for it in a goroutine to prevent zombie processes
	if err := cmd.Start(); err != nil {
		return
	}
	go cmd.Wait()
}
