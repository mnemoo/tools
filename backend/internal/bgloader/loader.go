// Package bgloader provides background loading of event books with priority control.
package bgloader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"lutexplorer/internal/lut"
	"lutexplorer/internal/ws"

	"github.com/klauspost/compress/zstd"
	"stakergs"
)

// Priority defines the loading priority level.
type Priority int

const (
	PriorityLow  Priority = iota // Slow background loading
	PriorityHigh                 // Fast loading (full CPU)
)

func (p Priority) String() string {
	if p == PriorityHigh {
		return "high"
	}
	return "low"
}

// ModeStatus represents the loading status of a mode.
type ModeStatus struct {
	Mode         string  `json:"mode"`
	EventsFile   string  `json:"events_file"`
	Status       string  `json:"status"` // "pending", "loading", "complete", "error"
	CurrentLine  int     `json:"current_line"`
	TotalLines   int     `json:"total_lines,omitempty"`
	BytesRead    int64   `json:"bytes_read"`
	TotalBytes   int64   `json:"total_bytes"`
	PercentBytes float64 `json:"percent_bytes"`
	Error        string  `json:"error,omitempty"`
	StartedAt    int64   `json:"started_at,omitempty"`
	CompletedAt  int64   `json:"completed_at,omitempty"`
}

// BackgroundLoader handles background loading of event books.
type BackgroundLoader struct {
	loader       *lut.Loader
	hub          *ws.Hub
	baseDir      string
	priority     atomic.Int32 // Current priority level
	modeStatuses map[string]*ModeStatus
	mu           sync.RWMutex
	stopCh       chan struct{}
	wg           sync.WaitGroup

	// How often to yield CPU in low priority mode (every N lines)
	lowPriorityBatchSize int
	// Delay after each batch in low priority mode
	lowPriorityBatchDelay time.Duration
	// How often to send progress updates
	progressInterval int // Every N lines
}

// NewBackgroundLoader creates a new background loader.
func NewBackgroundLoader(loader *lut.Loader, hub *ws.Hub) *BackgroundLoader {
	bl := &BackgroundLoader{
		loader:                loader,
		hub:                   hub,
		baseDir:               loader.BaseDir(),
		modeStatuses:          make(map[string]*ModeStatus),
		stopCh:                make(chan struct{}),
		lowPriorityBatchSize:  1000,                 // Process 1000 lines then yield
		lowPriorityBatchDelay: 1 * time.Millisecond, // Short pause after batch (~50% CPU)
		progressInterval:      1000,                 // Update every 1000 lines
	}
	bl.priority.Store(int32(PriorityLow))
	return bl
}

// Start begins background loading of all modes.
func (bl *BackgroundLoader) Start() {
	index := bl.loader.GetIndex()
	if index == nil {
		log.Println("BackgroundLoader: No index loaded")
		return
	}

	// Initialize status for all modes
	bl.mu.Lock()
	for _, mode := range index.Modes {
		if mode.Events != "" {
			bl.modeStatuses[mode.Name] = &ModeStatus{
				Mode:       mode.Name,
				EventsFile: mode.Events,
				Status:     "pending",
			}
		}
	}
	bl.mu.Unlock()

	// Start loading goroutine
	bl.wg.Add(1)
	go bl.loadAllModes(index.Modes)
}

// Stop stops all background loading.
func (bl *BackgroundLoader) Stop() {
	close(bl.stopCh)
	bl.wg.Wait()
}

// Restart stops current loading, reloads the index, and starts loading again.
func (bl *BackgroundLoader) Restart() error {
	// Stop any current loading
	select {
	case <-bl.stopCh:
		// Already stopped
	default:
		close(bl.stopCh)
	}
	bl.wg.Wait()

	// Reload index and LUT files
	if err := bl.loader.Reload(); err != nil {
		return err
	}

	// Update base dir in case it changed
	bl.baseDir = bl.loader.BaseDir()

	// Reset state
	bl.mu.Lock()
	bl.modeStatuses = make(map[string]*ModeStatus)
	bl.stopCh = make(chan struct{})
	bl.mu.Unlock()

	// Broadcast reload started
	bl.hub.Broadcast(ws.Message{
		Type: ws.MsgReloadStarted,
		Payload: map[string]string{
			"message": "Books and index reloaded, restarting background loading",
		},
	})

	// Start loading again
	bl.Start()

	return nil
}

// ReloadMode reloads events for a specific mode (used by file watcher).
func (bl *BackgroundLoader) ReloadMode(modeName string) error {
	index := bl.loader.GetIndex()
	if index == nil {
		return fmt.Errorf("no index loaded")
	}

	// Find mode config
	var modeConfig *stakergs.ModeConfig
	for i := range index.Modes {
		if index.Modes[i].Name == modeName {
			modeConfig = &index.Modes[i]
			break
		}
	}

	if modeConfig == nil {
		return fmt.Errorf("mode %q not found in index", modeName)
	}

	if modeConfig.Events == "" {
		return fmt.Errorf("mode %q has no events file", modeName)
	}

	// Clear existing events for this mode
	bl.loader.EventsLoader().ClearMode(modeName)

	// Reset status
	bl.mu.Lock()
	bl.modeStatuses[modeName] = &ModeStatus{
		Mode:       modeName,
		EventsFile: modeConfig.Events,
		Status:     "pending",
	}
	bl.mu.Unlock()

	// Broadcast reload started for this mode
	bl.hub.Broadcast(ws.Message{
		Type: ws.MsgReloadStarted,
		Mode: modeName,
		Payload: map[string]string{
			"mode":    modeName,
			"message": fmt.Sprintf("Reloading events for mode %s (file changed)", modeName),
		},
	})

	// Load mode in background with retry
	go func() {
		bl.loadModeWithRetry(*modeConfig, 3)
	}()

	return nil
}

// loadModeWithRetry attempts to load a mode with retries on failure.
// This handles cases where the file might still be incomplete.
func (bl *BackgroundLoader) loadModeWithRetry(mode stakergs.ModeConfig, maxRetries int) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * 2 * time.Second
			log.Printf("BackgroundLoader: Retry %d/%d for mode %q in %v", attempt, maxRetries, mode.Name, delay)
			time.Sleep(delay)

			// Reset status for retry
			bl.mu.Lock()
			bl.modeStatuses[mode.Name] = &ModeStatus{
				Mode:       mode.Name,
				EventsFile: mode.Events,
				Status:     "pending",
			}
			bl.mu.Unlock()
		}

		// Try to load the mode
		err := bl.loadModeInternal(mode)
		if err == nil {
			return // Success
		}

		lastErr = err
		log.Printf("BackgroundLoader: Attempt %d failed for mode %q: %v", attempt+1, mode.Name, err)
	}

	// All retries failed
	bl.setModeError(mode.Name, fmt.Sprintf("failed after %d attempts: %v", maxRetries+1, lastErr))
}

// GetModeEventsFile returns the events file for a mode.
func (bl *BackgroundLoader) GetModeEventsFile(modeName string) string {
	index := bl.loader.GetIndex()
	if index == nil {
		return ""
	}
	for _, mode := range index.Modes {
		if mode.Name == modeName {
			return mode.Events
		}
	}
	return ""
}

// GetBookFiles returns a map of event filenames to mode names.
func (bl *BackgroundLoader) GetBookFiles() map[string]string {
	index := bl.loader.GetIndex()
	if index == nil {
		return nil
	}
	books := make(map[string]string)
	for _, mode := range index.Modes {
		if mode.Events != "" {
			books[mode.Events] = mode.Name
		}
	}
	return books
}

// SetPriority sets the loading priority.
func (bl *BackgroundLoader) SetPriority(p Priority) {
	old := Priority(bl.priority.Swap(int32(p)))
	if old != p {
		log.Printf("BackgroundLoader: Priority changed from %s to %s", old, p)
		bl.hub.Broadcast(ws.Message{
			Type: ws.MsgPriorityChanged,
			Payload: map[string]string{
				"old_priority": old.String(),
				"new_priority": p.String(),
			},
		})
	}
}

// GetPriority returns the current loading priority.
func (bl *BackgroundLoader) GetPriority() Priority {
	return Priority(bl.priority.Load())
}

// GetStatus returns the current status of all modes.
func (bl *BackgroundLoader) GetStatus() map[string]*ModeStatus {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	result := make(map[string]*ModeStatus, len(bl.modeStatuses))
	for k, v := range bl.modeStatuses {
		// Make a copy
		statusCopy := *v
		result[k] = &statusCopy
	}
	return result
}

// GetModeStatus returns the status of a specific mode.
func (bl *BackgroundLoader) GetModeStatus(mode string) *ModeStatus {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	if status, ok := bl.modeStatuses[mode]; ok {
		statusCopy := *status
		return &statusCopy
	}
	return nil
}

// loadAllModes loads events for all modes sequentially.
func (bl *BackgroundLoader) loadAllModes(modes []stakergs.ModeConfig) {
	defer bl.wg.Done()

	for _, mode := range modes {
		if mode.Events == "" {
			continue
		}

		select {
		case <-bl.stopCh:
			return
		default:
		}

		// Skip if already loaded
		if bl.loader.EventsLoader().IsLoaded(mode.Name) {
			bl.mu.Lock()
			if status, ok := bl.modeStatuses[mode.Name]; ok {
				status.Status = "complete"
				status.PercentBytes = 100
			}
			bl.mu.Unlock()
			continue
		}

		bl.loadMode(mode)
	}

	log.Println("BackgroundLoader: All modes loaded")
}

// loadMode loads events for a single mode (wrapper for backwards compatibility).
func (bl *BackgroundLoader) loadMode(mode stakergs.ModeConfig) {
	if err := bl.loadModeInternal(mode); err != nil {
		bl.setModeError(mode.Name, err.Error())
	}
}

// loadModeInternal loads events for a single mode with progress tracking.
// Returns an error if loading fails (e.g., EOF, corrupt file).
func (bl *BackgroundLoader) loadModeInternal(mode stakergs.ModeConfig) error {
	filePath := filepath.Join(bl.baseDir, mode.Events)

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	totalBytes := fileInfo.Size()

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Wrap file in a counting reader
	countingReader := &countingReader{reader: file}

	// Create zstd decoder
	decoder, err := zstd.NewReader(countingReader)
	if err != nil {
		return fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer decoder.Close()

	// Update status to loading
	startTime := time.Now()
	bl.mu.Lock()
	if status, ok := bl.modeStatuses[mode.Name]; ok {
		status.Status = "loading"
		status.TotalBytes = totalBytes
		status.StartedAt = startTime.UnixMilli()
	}
	bl.mu.Unlock()

	// Broadcast loading started
	bl.hub.Broadcast(ws.Message{
		Type: ws.MsgLoadingStarted,
		Mode: mode.Name,
		Payload: map[string]interface{}{
			"mode":        mode.Name,
			"events_file": mode.Events,
			"total_bytes": totalBytes,
		},
	})

	// Read events line by line
	events := make(map[int]json.RawMessage)
	scanner := bufio.NewScanner(decoder)
	const maxCapacity = 10 * 1024 * 1024 // 10MB buffer
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	lastProgressUpdate := time.Now()

	for scanner.Scan() {
		select {
		case <-bl.stopCh:
			return fmt.Errorf("loading stopped")
		default:
		}

		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Copy event data
		eventCopy := make(json.RawMessage, len(line))
		copy(eventCopy, line)
		events[lineNum] = eventCopy

		// Send progress update
		if lineNum%bl.progressInterval == 0 || time.Since(lastProgressUpdate) > 500*time.Millisecond {
			bytesRead := countingReader.BytesRead()
			elapsed := time.Since(startTime)
			linesPerSec := float64(lineNum) / elapsed.Seconds()

			progress := ws.LoadingProgress{
				Mode:           mode.Name,
				EventsFile:     mode.Events,
				CurrentLine:    lineNum,
				BytesRead:      bytesRead,
				TotalBytes:     totalBytes,
				PercentBytes:   float64(bytesRead) / float64(totalBytes) * 100,
				Priority:       bl.GetPriority().String(),
				ElapsedMs:      elapsed.Milliseconds(),
				LinesPerSecond: linesPerSec,
			}

			// Estimate remaining time
			if bytesRead > 0 && bytesRead < totalBytes {
				remainingBytes := totalBytes - bytesRead
				bytesPerSec := float64(bytesRead) / elapsed.Seconds()
				if bytesPerSec > 0 {
					progress.EstimatedMs = int64(float64(remainingBytes)/bytesPerSec) * 1000
				}
			}

			bl.mu.Lock()
			if status, ok := bl.modeStatuses[mode.Name]; ok {
				status.CurrentLine = lineNum
				status.BytesRead = bytesRead
				status.PercentBytes = progress.PercentBytes
			}
			bl.mu.Unlock()

			bl.hub.Broadcast(ws.Message{
				Type:    ws.MsgLoadingProgress,
				Mode:    mode.Name,
				Payload: progress,
			})

			lastProgressUpdate = time.Now()
		}

		// Yield CPU periodically in low priority mode (batch-based for efficiency)
		if bl.GetPriority() == PriorityLow && lineNum%bl.lowPriorityBatchSize == 0 {
			time.Sleep(bl.lowPriorityBatchDelay)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading events: %w", err)
	}

	// Store events in the loader
	bl.loader.EventsLoader().SetEvents(mode.Name, events, filePath)

	// Update status to complete
	completedAt := time.Now()
	bl.mu.Lock()
	if status, ok := bl.modeStatuses[mode.Name]; ok {
		status.Status = "complete"
		status.CurrentLine = lineNum
		status.TotalLines = lineNum
		status.BytesRead = countingReader.BytesRead()
		status.PercentBytes = 100
		status.CompletedAt = completedAt.UnixMilli()
	}
	bl.mu.Unlock()

	// Broadcast loading complete
	elapsed := completedAt.Sub(startTime)
	bl.hub.Broadcast(ws.Message{
		Type: ws.MsgLoadingComplete,
		Mode: mode.Name,
		Payload: map[string]interface{}{
			"mode":         mode.Name,
			"total_lines":  lineNum,
			"total_bytes":  countingReader.BytesRead(),
			"elapsed_ms":   elapsed.Milliseconds(),
			"lines_per_sec": float64(lineNum) / elapsed.Seconds(),
		},
	})

	log.Printf("BackgroundLoader: Loaded %d events for mode %q in %v", lineNum, mode.Name, elapsed)

	return nil
}

// setModeError sets an error status for a mode.
func (bl *BackgroundLoader) setModeError(mode string, errMsg string) {
	bl.mu.Lock()
	if status, ok := bl.modeStatuses[mode]; ok {
		status.Status = "error"
		status.Error = errMsg
	}
	bl.mu.Unlock()

	bl.hub.Broadcast(ws.Message{
		Type: ws.MsgLoadingError,
		Mode: mode,
		Payload: map[string]string{
			"mode":  mode,
			"error": errMsg,
		},
	})

	log.Printf("BackgroundLoader: Error loading mode %q: %s", mode, errMsg)
}

// countingReader wraps a reader and counts bytes read.
type countingReader struct {
	reader    io.Reader
	bytesRead int64
	mu        sync.Mutex
}

func (cr *countingReader) Read(p []byte) (n int, err error) {
	n, err = cr.reader.Read(p)
	cr.mu.Lock()
	cr.bytesRead += int64(n)
	cr.mu.Unlock()
	return
}

func (cr *countingReader) BytesRead() int64 {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	return cr.bytesRead
}
