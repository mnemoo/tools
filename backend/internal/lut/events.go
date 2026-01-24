package lut

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// EventsLoader handles loading and decompressing event files (.jsonl.zst).
type EventsLoader struct {
	baseDir string
	cache   map[string]*EventsIndex // mode -> events index (full load, legacy)
	chunks  map[string]*ChunkCache  // mode -> chunk cache (lazy loading)
	mu      sync.RWMutex            // protects cache from concurrent access
}

// ChunkCache holds cached event chunks for lazy loading.
type ChunkCache struct {
	Mode      string
	FilePath  string
	Chunks    map[int]map[int]json.RawMessage // chunkID -> (lineIndex -> event)
	ChunkSize int                             // lines per chunk
	MaxChunks int                             // max chunks to keep in memory
	LRU       []int                           // chunk IDs in LRU order (oldest first)
	mu        sync.RWMutex
}

const (
	DefaultChunkSize = 1000 // 1000 lines per chunk
	DefaultMaxChunks = 10   // Keep max 10 chunks in memory (~10k events)
)

// EventsIndex holds indexed events for fast lookup by sim_id.
type EventsIndex struct {
	Mode     string
	FilePath string
	Events   map[int]json.RawMessage // sim_id -> raw JSON event
	Count    int
}

// EventInfo contains event data with statistics.
type EventInfo struct {
	SimID       int             `json:"sim_id"`
	Event       json.RawMessage `json:"event"`
	Weight      uint64          `json:"weight"`
	Payout      float64         `json:"payout"`
	Probability float64         `json:"probability"`
	Odds        string          `json:"odds"`
}

// NewEventsLoader creates a new events loader.
func NewEventsLoader(baseDir string) *EventsLoader {
	return &EventsLoader{
		baseDir: baseDir,
		cache:   make(map[string]*EventsIndex),
		chunks:  make(map[string]*ChunkCache),
	}
}

// findMode does case-insensitive lookup for mode in cache.
// IMPORTANT: caller must hold at least e.mu.RLock()
func (e *EventsLoader) findModeLocked(mode string) (*EventsIndex, bool) {
	modeLower := strings.ToLower(mode)
	for name, index := range e.cache {
		if strings.ToLower(name) == modeLower {
			return index, true
		}
	}
	return nil, false
}

// LoadEvents loads and indexes events from a .jsonl.zst file.
func (e *EventsLoader) LoadEvents(mode, eventsFile string) error {
	filePath := filepath.Join(e.baseDir, eventsFile)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open events file: %w", err)
	}
	defer file.Close()

	// Create zstd decoder
	decoder, err := zstd.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer decoder.Close()

	// Read and index events
	index := &EventsIndex{
		Mode:     mode,
		FilePath: filePath,
		Events:   make(map[int]json.RawMessage),
	}

	scanner := bufio.NewScanner(decoder)
	// Increase buffer size for large JSON lines
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			lineNum++
			continue
		}

		// sim_id = line index (0-indexed, matches CSV sim_id)
		eventCopy := make(json.RawMessage, len(line))
		copy(eventCopy, line)
		index.Events[lineNum] = eventCopy
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading events: %w", err)
	}

	index.Count = len(index.Events)

	e.mu.Lock()
	e.cache[mode] = index
	e.mu.Unlock()

	return nil
}

// GetEvent retrieves a single event by sim_id (case-insensitive mode lookup).
// simIDOffset is the minimum sim_id from the LUT (0 or 1) for backwards compatibility.
func (e *EventsLoader) GetEvent(mode string, simID int, simIDOffset int) (json.RawMessage, error) {
	e.mu.RLock()
	index, ok := e.findModeLocked(mode)
	if !ok {
		e.mu.RUnlock()
		return nil, fmt.Errorf("events for mode %q not loaded", mode)
	}

	// Convert sim_id to 0-indexed event position
	// Old format: sim_id starts from 1, so eventIndex = simID - 1
	// New format: sim_id starts from 0, so eventIndex = simID - 0
	eventIndex := simID - simIDOffset
	event, ok := index.Events[eventIndex]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("event with sim_id %d (index %d) not found in mode %q", simID, eventIndex, mode)
	}

	return event, nil
}

// GetEventInfo retrieves event with full statistics.
// simIDOffset is the minimum sim_id from the LUT (0 or 1) for backwards compatibility.
func (e *EventsLoader) GetEventInfo(mode string, simID int, simIDOffset int, outcome *OutcomeStats) (*EventInfo, error) {
	event, err := e.GetEvent(mode, simID, simIDOffset)
	if err != nil {
		return nil, err
	}

	info := &EventInfo{
		SimID: simID,
		Event: event,
	}

	if outcome != nil {
		info.Weight = outcome.Weight
		info.Payout = outcome.Payout
		info.Probability = outcome.Probability
		info.Odds = outcome.Odds
	}

	return info, nil
}

// IsLoaded checks if events for a mode are loaded (case-insensitive).
func (e *EventsLoader) IsLoaded(mode string) bool {
	e.mu.RLock()
	_, ok := e.findModeLocked(mode)
	e.mu.RUnlock()
	return ok
}

// GetLoadedModes returns list of modes with loaded events.
func (e *EventsLoader) GetLoadedModes() []string {
	e.mu.RLock()
	modes := make([]string, 0, len(e.cache))
	for mode := range e.cache {
		modes = append(modes, mode)
	}
	e.mu.RUnlock()
	return modes
}

// GetEventCount returns number of events for a mode (case-insensitive).
func (e *EventsLoader) GetEventCount(mode string) int {
	e.mu.RLock()
	index, ok := e.findModeLocked(mode)
	e.mu.RUnlock()
	if ok {
		return index.Count
	}
	return 0
}

// SetEvents stores events loaded by the background loader.
func (e *EventsLoader) SetEvents(mode string, events map[int]json.RawMessage, filePath string) {
	e.mu.Lock()
	e.cache[mode] = &EventsIndex{
		Mode:     mode,
		FilePath: filePath,
		Events:   events,
		Count:    len(events),
	}
	e.mu.Unlock()
}

// ClearAll removes all cached events.
func (e *EventsLoader) ClearAll() {
	e.mu.Lock()
	e.cache = make(map[string]*EventsIndex)
	e.mu.Unlock()
}

// ClearMode removes cached events for a specific mode.
func (e *EventsLoader) ClearMode(mode string) {
	e.mu.Lock()
	delete(e.cache, mode)
	e.mu.Unlock()
}

// StreamEvents streams events through a callback (for large files).
// lineIndex passed to callback is 0-indexed to match CSV sim_id format.
func (e *EventsLoader) StreamEvents(eventsFile string, callback func(lineIndex int, event json.RawMessage) error) error {
	filePath := filepath.Join(e.baseDir, eventsFile)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open events file: %w", err)
	}
	defer file.Close()

	decoder, err := zstd.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer decoder.Close()

	reader := bufio.NewReader(decoder)
	lineIndex := 0

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				if len(line) > 0 {
					if err := callback(lineIndex, line); err != nil {
						return err
					}
				}
				break
			}
			return fmt.Errorf("error reading line %d: %w", lineIndex, err)
		}

		if len(line) > 0 {
			if err := callback(lineIndex, line); err != nil {
				return err
			}
		}
		lineIndex++
	}

	return nil
}

// OutcomeStats holds statistics for an outcome.
type OutcomeStats struct {
	SimID       int
	Weight      uint64
	Payout      float64
	Probability float64
	Odds        string
}

// FormatOdds formats probability as odds string.
func FormatOdds(probability float64) string {
	if probability == 0 {
		return "-"
	}
	odds := 1 / probability
	if odds >= 1000000 {
		return fmt.Sprintf("1 in %.1fM", odds/1000000)
	}
	if odds >= 1000 {
		return fmt.Sprintf("1 in %.1fK", odds/1000)
	}
	return fmt.Sprintf("1 in %.0f", odds)
}

// ============================================================================
// Lazy Loading Methods - Load only what's needed, unload when done
// ============================================================================

// GetEventsRange loads events for a specific line range [startLine, endLine).
// This streams through the file and only keeps the requested range in memory.
// Returns a map of lineIndex -> event.
func (e *EventsLoader) GetEventsRange(eventsFile string, startLine, endLine int) (map[int]json.RawMessage, error) {
	filePath := filepath.Join(e.baseDir, eventsFile)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open events file: %w", err)
	}
	defer file.Close()

	decoder, err := zstd.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer decoder.Close()

	scanner := bufio.NewScanner(decoder)
	// Use smaller buffer for range loading (1MB instead of 10MB)
	const maxCapacity = 1 * 1024 * 1024
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	events := make(map[int]json.RawMessage)
	lineIndex := 0

	for scanner.Scan() {
		// Skip lines before range
		if lineIndex < startLine {
			lineIndex++
			continue
		}
		// Stop after range
		if lineIndex >= endLine {
			break
		}

		line := scanner.Bytes()
		if len(line) > 0 {
			eventCopy := make(json.RawMessage, len(line))
			copy(eventCopy, line)
			events[lineIndex] = eventCopy
		}
		lineIndex++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading events: %w", err)
	}

	return events, nil
}

// GetEventLazy gets a single event using chunk-based lazy loading.
// It loads a chunk around the requested line and caches it.
func (e *EventsLoader) GetEventLazy(mode, eventsFile string, lineIndex int, simIDOffset int) (json.RawMessage, error) {
	// First check if we have it in full cache (legacy)
	e.mu.RLock()
	if index, ok := e.findModeLocked(mode); ok {
		eventIdx := lineIndex - simIDOffset
		if event, ok := index.Events[eventIdx]; ok {
			e.mu.RUnlock()
			return event, nil
		}
	}
	e.mu.RUnlock()

	// Use chunk cache for lazy loading
	cache := e.getOrCreateChunkCache(mode, eventsFile)

	chunkID := lineIndex / cache.ChunkSize
	eventIdx := lineIndex - simIDOffset

	// Check if chunk is cached
	cache.mu.RLock()
	if chunk, ok := cache.Chunks[chunkID]; ok {
		if event, ok := chunk[eventIdx]; ok {
			cache.mu.RUnlock()
			cache.touchChunk(chunkID)
			return event, nil
		}
	}
	cache.mu.RUnlock()

	// Load the chunk
	startLine := chunkID * cache.ChunkSize
	endLine := startLine + cache.ChunkSize

	events, err := e.GetEventsRange(eventsFile, startLine, endLine)
	if err != nil {
		return nil, err
	}

	// Store in cache
	cache.mu.Lock()
	cache.Chunks[chunkID] = events
	cache.touchChunkLocked(chunkID)
	cache.evictIfNeededLocked()
	cache.mu.Unlock()

	// Return requested event
	if event, ok := events[eventIdx]; ok {
		return event, nil
	}
	return nil, fmt.Errorf("event at line %d not found", lineIndex)
}

// getOrCreateChunkCache returns or creates a chunk cache for a mode.
func (e *EventsLoader) getOrCreateChunkCache(mode, eventsFile string) *ChunkCache {
	e.mu.Lock()
	defer e.mu.Unlock()

	modeLower := strings.ToLower(mode)
	if cache, ok := e.chunks[modeLower]; ok {
		return cache
	}

	cache := &ChunkCache{
		Mode:      mode,
		FilePath:  filepath.Join(e.baseDir, eventsFile),
		Chunks:    make(map[int]map[int]json.RawMessage),
		ChunkSize: DefaultChunkSize,
		MaxChunks: DefaultMaxChunks,
		LRU:       make([]int, 0),
	}
	e.chunks[modeLower] = cache
	return cache
}

// touchChunk moves a chunk to the end of LRU (most recently used).
func (c *ChunkCache) touchChunk(chunkID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.touchChunkLocked(chunkID)
}

func (c *ChunkCache) touchChunkLocked(chunkID int) {
	// Remove from current position
	for i, id := range c.LRU {
		if id == chunkID {
			c.LRU = append(c.LRU[:i], c.LRU[i+1:]...)
			break
		}
	}
	// Add to end (most recent)
	c.LRU = append(c.LRU, chunkID)
}

// evictIfNeededLocked removes oldest chunks if over limit.
// Caller must hold c.mu.Lock().
func (c *ChunkCache) evictIfNeededLocked() {
	for len(c.LRU) > c.MaxChunks {
		oldestID := c.LRU[0]
		c.LRU = c.LRU[1:]
		delete(c.Chunks, oldestID)
	}
}

// UnloadMode removes all cached events for a mode (both full and chunks).
func (e *EventsLoader) UnloadMode(mode string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	modeLower := strings.ToLower(mode)

	// Clear full cache
	for name := range e.cache {
		if strings.ToLower(name) == modeLower {
			delete(e.cache, name)
			break
		}
	}

	// Clear chunk cache
	delete(e.chunks, modeLower)
}

// UnloadAll removes all cached events.
func (e *EventsLoader) UnloadAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cache = make(map[string]*EventsIndex)
	e.chunks = make(map[string]*ChunkCache)
}

// GetChunkCacheStats returns stats about chunk cache for debugging.
func (e *EventsLoader) GetChunkCacheStats(mode string) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	modeLower := strings.ToLower(mode)
	cache, ok := e.chunks[modeLower]
	if !ok {
		return nil
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	totalEvents := 0
	for _, chunk := range cache.Chunks {
		totalEvents += len(chunk)
	}

	return map[string]interface{}{
		"mode":         cache.Mode,
		"chunks":       len(cache.Chunks),
		"max_chunks":   cache.MaxChunks,
		"chunk_size":   cache.ChunkSize,
		"total_events": totalEvents,
		"lru_order":    cache.LRU,
	}
}
