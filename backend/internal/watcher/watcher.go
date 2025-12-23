// Package watcher provides file system watching for book files.
// When book files are modified, it triggers automatic reload of events.
package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ReloadFunc is called when a book file changes.
// mode is the game mode name (e.g., "base", "bonus").
type ReloadFunc func(mode string) error

// BookWatcher watches book files for changes and triggers reloads.
type BookWatcher struct {
	watcher     *fsnotify.Watcher
	baseDir     string
	bookFiles   map[string]string // filename -> mode name
	onReload    ReloadFunc
	debounce    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
	lastChange  map[string]time.Time // debounce tracking
}

// NewBookWatcher creates a new watcher for book files.
// bookFiles maps event filenames to their mode names.
// Example: {"books_base.jsonl.zst": "base", "books_bonus.jsonl.zst": "bonus"}
func NewBookWatcher(baseDir string, bookFiles map[string]string, onReload ReloadFunc) (*BookWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &BookWatcher{
		watcher:    w,
		baseDir:    baseDir,
		bookFiles:  bookFiles,
		onReload:   onReload,
		debounce:   2 * time.Second, // debounce rapid changes
		stopCh:     make(chan struct{}),
		lastChange: make(map[string]time.Time),
	}, nil
}

// Start begins watching for file changes.
func (bw *BookWatcher) Start() error {
	// Watch the base directory
	if err := bw.watcher.Add(bw.baseDir); err != nil {
		return err
	}

	log.Printf("[Watcher] Watching directory: %s", bw.baseDir)
	for filename := range bw.bookFiles {
		log.Printf("[Watcher] Tracking book file: %s", filename)
	}

	bw.wg.Add(1)
	go bw.run()

	return nil
}

// Stop stops watching for file changes.
func (bw *BookWatcher) Stop() {
	close(bw.stopCh)
	bw.watcher.Close()
	bw.wg.Wait()
	log.Println("[Watcher] Stopped")
}

func (bw *BookWatcher) run() {
	defer bw.wg.Done()

	for {
		select {
		case <-bw.stopCh:
			return

		case event, ok := <-bw.watcher.Events:
			if !ok {
				return
			}
			bw.handleEvent(event)

		case err, ok := <-bw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[Watcher] Error: %v", err)
		}
	}
}

func (bw *BookWatcher) handleEvent(event fsnotify.Event) {
	// Only care about write and create events
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return
	}

	filename := filepath.Base(event.Name)

	// Check if this is a book file we're tracking
	mode, ok := bw.bookFiles[filename]
	if !ok {
		return
	}

	// Debounce: ignore if last change was too recent
	bw.mu.Lock()
	lastTime, exists := bw.lastChange[filename]
	now := time.Now()
	if exists && now.Sub(lastTime) < bw.debounce {
		bw.mu.Unlock()
		return
	}
	bw.lastChange[filename] = now
	bw.mu.Unlock()

	log.Printf("[Watcher] Book file changed: %s (mode: %s)", filename, mode)

	// Trigger reload in a goroutine to not block the watcher
	go func(m string, f string, fullPath string) {
		// Wait for file to stabilize (stop being written to)
		// This is crucial for large files that take time to write
		if err := bw.waitForFileStable(fullPath); err != nil {
			log.Printf("[Watcher] File %s not stable, skipping reload: %v", f, err)
			return
		}

		log.Printf("[Watcher] Reloading events for mode: %s", m)
		if err := bw.onReload(m); err != nil {
			log.Printf("[Watcher] Failed to reload mode %s: %v", m, err)
		} else {
			log.Printf("[Watcher] Successfully reloaded mode: %s", m)
		}
	}(mode, filename, event.Name)
}

// waitForFileStable waits until the file size stops changing.
// This prevents reading a file that is still being written.
func (bw *BookWatcher) waitForFileStable(path string) error {
	const (
		checkInterval  = 200 * time.Millisecond // How often to check file size
		stableRequired = 3                       // Number of consecutive stable checks required
		maxWait        = 30 * time.Second        // Maximum wait time
	)

	startTime := time.Now()
	var lastSize int64 = -1
	stableCount := 0

	for {
		if time.Since(startTime) > maxWait {
			log.Printf("[Watcher] File %s: max wait time exceeded, proceeding anyway", filepath.Base(path))
			return nil // Proceed anyway after max wait
		}

		info, err := os.Stat(path)
		if err != nil {
			// File might be temporarily unavailable during write
			time.Sleep(checkInterval)
			stableCount = 0
			lastSize = -1
			continue
		}

		currentSize := info.Size()

		if currentSize == lastSize && currentSize > 0 {
			stableCount++
			if stableCount >= stableRequired {
				log.Printf("[Watcher] File %s stable at %d bytes after %v",
					filepath.Base(path), currentSize, time.Since(startTime))
				return nil
			}
		} else {
			stableCount = 0
		}

		lastSize = currentSize
		time.Sleep(checkInterval)
	}
}

// SetDebounce sets the debounce duration for file changes.
func (bw *BookWatcher) SetDebounce(d time.Duration) {
	bw.mu.Lock()
	bw.debounce = d
	bw.mu.Unlock()
}

// AddBookFile adds a new book file to watch.
func (bw *BookWatcher) AddBookFile(filename, mode string) {
	bw.mu.Lock()
	bw.bookFiles[filename] = mode
	bw.mu.Unlock()
	log.Printf("[Watcher] Added book file: %s (mode: %s)", filename, mode)
}

// ExtractBookFiles extracts book files from mode configurations.
// Returns a map of filename -> mode name.
func ExtractBookFiles(modes []struct {
	Name   string
	Events string
}) map[string]string {
	books := make(map[string]string)
	for _, m := range modes {
		if m.Events != "" && strings.HasSuffix(m.Events, ".zst") {
			books[m.Events] = m.Name
		}
	}
	return books
}
