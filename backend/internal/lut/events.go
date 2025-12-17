package lut

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// EventsLoader handles loading and decompressing event files (.jsonl.zst).
type EventsLoader struct {
	baseDir string
	cache   map[string]*EventsIndex // mode -> events index
}

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
	}
}

// findMode does case-insensitive lookup for mode in cache
func (e *EventsLoader) findMode(mode string) (*EventsIndex, bool) {
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
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// sim_id = line number (1-indexed)
		eventCopy := make(json.RawMessage, len(line))
		copy(eventCopy, line)
		index.Events[lineNum] = eventCopy
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading events: %w", err)
	}

	index.Count = len(index.Events)
	e.cache[mode] = index

	return nil
}

// GetEvent retrieves a single event by sim_id (case-insensitive mode lookup).
func (e *EventsLoader) GetEvent(mode string, simID int) (json.RawMessage, error) {
	index, ok := e.findMode(mode)
	if !ok {
		return nil, fmt.Errorf("events for mode %q not loaded", mode)
	}

	event, ok := index.Events[simID]
	if !ok {
		return nil, fmt.Errorf("event with sim_id %d not found in mode %q", simID, mode)
	}

	return event, nil
}

// GetEventInfo retrieves event with full statistics.
func (e *EventsLoader) GetEventInfo(mode string, simID int, outcome *OutcomeStats) (*EventInfo, error) {
	event, err := e.GetEvent(mode, simID)
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
	_, ok := e.findMode(mode)
	return ok
}

// GetLoadedModes returns list of modes with loaded events.
func (e *EventsLoader) GetLoadedModes() []string {
	modes := make([]string, 0, len(e.cache))
	for mode := range e.cache {
		modes = append(modes, mode)
	}
	return modes
}

// GetEventCount returns number of events for a mode (case-insensitive).
func (e *EventsLoader) GetEventCount(mode string) int {
	if index, ok := e.findMode(mode); ok {
		return index.Count
	}
	return 0
}

// SetEvents stores events loaded by the background loader.
func (e *EventsLoader) SetEvents(mode string, events map[int]json.RawMessage, filePath string) {
	e.cache[mode] = &EventsIndex{
		Mode:     mode,
		FilePath: filePath,
		Events:   events,
		Count:    len(events),
	}
}

// ClearAll removes all cached events.
func (e *EventsLoader) ClearAll() {
	e.cache = make(map[string]*EventsIndex)
}

// ClearMode removes cached events for a specific mode.
func (e *EventsLoader) ClearMode(mode string) {
	delete(e.cache, mode)
}

// StreamEvents streams events through a callback (for large files).
func (e *EventsLoader) StreamEvents(eventsFile string, callback func(lineNum int, event json.RawMessage) error) error {
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
	lineNum := 0

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				if len(line) > 0 {
					lineNum++
					if err := callback(lineNum, line); err != nil {
						return err
					}
				}
				break
			}
			return fmt.Errorf("error reading line %d: %w", lineNum+1, err)
		}

		lineNum++
		if len(line) > 0 {
			if err := callback(lineNum, line); err != nil {
				return err
			}
		}
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
