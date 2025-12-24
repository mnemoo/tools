package lut

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"stakergs"
)

// Loader handles loading and caching of LUT index files.
type Loader struct {
	indexPath         string
	baseDir           string
	index             *stakergs.GameIndex
	tables            map[string]*stakergs.LookupTable
	analyzer          *Analyzer
	eventsLoader      *EventsLoader
	simulator         *Simulator
	distributionCache *DistributionCache
}

// NewLoader creates a new LUT loader for the given index file path.
func NewLoader(indexPath string) *Loader {
	baseDir := filepath.Dir(indexPath)
	return &Loader{
		indexPath:         indexPath,
		baseDir:           baseDir,
		tables:            make(map[string]*stakergs.LookupTable),
		analyzer:          NewAnalyzer(),
		eventsLoader:      NewEventsLoader(baseDir),
		simulator:         NewSimulator(),
		distributionCache: NewDistributionCache(),
	}
}

// Simulator returns the LUT simulator.
func (l *Loader) Simulator() *Simulator {
	return l.simulator
}

// Load reads and parses the index.json file and all referenced LUT CSV files.
func (l *Loader) Load() error {
	absPath, err := filepath.Abs(l.indexPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	l.baseDir = filepath.Dir(absPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read index file: %w", err)
	}

	var index stakergs.GameIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("failed to parse index file: %w", err)
	}

	l.index = &index

	// Load all LUT CSV files
	for _, mode := range index.Modes {
		table, err := l.loadCSV(mode)
		if err != nil {
			return fmt.Errorf("failed to load LUT for mode %q: %w", mode.Name, err)
		}
		l.tables[mode.Name] = table
	}

	return nil
}

// loadCSV reads a LUT CSV file and returns a LookupTable.
// CSV format: sim_id,weight,payout (no header)
func (l *Loader) loadCSV(mode stakergs.ModeConfig) (*stakergs.LookupTable, error) {
	csvPath := filepath.Join(l.baseDir, mode.Weights)

	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV: %w", err)
	}
	defer file.Close()

	var outcomes []stakergs.Outcome
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil, fmt.Errorf("line %d: expected 3 fields, got %d", lineNum, len(parts))
		}

		simID, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid sim_id: %w", lineNum, err)
		}

		weight, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid weight: %w", lineNum, err)
		}

		payout, err := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid payout: %w", lineNum, err)
		}

		outcomes = append(outcomes, stakergs.Outcome{
			SimID:  simID,
			Weight: weight,
			Payout: uint(payout),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading CSV: %w", err)
	}

	// Determine SimIDOffset (minimum sim_id) for backwards compatibility
	// Old format: sim_id starts from 1, New format: sim_id starts from 0
	simIDOffset := 0
	if len(outcomes) > 0 {
		simIDOffset = outcomes[0].SimID
		for _, o := range outcomes {
			if o.SimID < simIDOffset {
				simIDOffset = o.SimID
			}
		}
	}

	return &stakergs.LookupTable{
		Outcomes:    outcomes,
		Mode:        mode.Name,
		Cost:        mode.Cost,
		SimIDOffset: simIDOffset,
	}, nil
}

// GetIndex returns the loaded game index.
func (l *Loader) GetIndex() *stakergs.GameIndex {
	return l.index
}

// GetMode returns a specific mode's lookup table.
func (l *Loader) GetMode(mode string) (*stakergs.LookupTable, error) {
	if l.index == nil {
		return nil, fmt.Errorf("index not loaded")
	}

	// Case-insensitive lookup
	modeLower := strings.ToLower(mode)
	for name, table := range l.tables {
		if strings.ToLower(name) == modeLower {
			return table, nil
		}
	}

	return nil, fmt.Errorf("mode %q not found", mode)
}

// ListModes returns all available mode names.
func (l *Loader) ListModes() []string {
	if l.index == nil {
		return nil
	}

	modes := make([]string, 0, len(l.index.Modes))
	for _, mode := range l.index.Modes {
		modes = append(modes, mode.Name)
	}
	return modes
}

// IndexPath returns the path to the loaded index file.
func (l *Loader) IndexPath() string {
	return l.indexPath
}

// Analyzer returns the LUT analyzer.
func (l *Loader) Analyzer() *Analyzer {
	return l.analyzer
}

// DistributionCache returns the distribution cache.
func (l *Loader) DistributionCache() *DistributionCache {
	return l.distributionCache
}

// ModeSummary contains basic info about a mode.
type ModeSummary struct {
	Mode      string  `json:"mode"`
	Cost      float64 `json:"cost"`
	Outcomes  int     `json:"outcomes"`
	RTP       float64 `json:"rtp"`
	HitRate   float64 `json:"hit_rate"`
	MaxPayout float64 `json:"max_payout"`
}

// GetModeSummaries returns summaries for all modes.
func (l *Loader) GetModeSummaries() []ModeSummary {
	if l.index == nil {
		return nil
	}

	summaries := make([]ModeSummary, 0, len(l.index.Modes))
	for _, mode := range l.index.Modes {
		table := l.tables[mode.Name]
		if table == nil {
			continue
		}
		summaries = append(summaries, ModeSummary{
			Mode:      mode.Name,
			Cost:      mode.Cost,
			Outcomes:  len(table.Outcomes),
			RTP:       table.RTP(),
			HitRate:   table.HitRate(),
			MaxPayout: float64(table.MaxPayout()) / 100.0,
		})
	}
	return summaries
}

// GetModeConfig returns the configuration for a specific mode.
func (l *Loader) GetModeConfig(mode string) (*stakergs.ModeConfig, error) {
	if l.index == nil {
		return nil, fmt.Errorf("index not loaded")
	}

	// Case-insensitive lookup
	modeLower := strings.ToLower(mode)
	for i := range l.index.Modes {
		if strings.ToLower(l.index.Modes[i].Name) == modeLower {
			return &l.index.Modes[i], nil
		}
	}
	return nil, fmt.Errorf("mode %q not found", mode)
}

// LoadEvents loads events for a specific mode.
func (l *Loader) LoadEvents(mode string) error {
	config, err := l.GetModeConfig(mode)
	if err != nil {
		return err
	}

	if config.Events == "" {
		return fmt.Errorf("mode %q has no events file configured", mode)
	}

	return l.eventsLoader.LoadEvents(mode, config.Events)
}

// EventsLoader returns the events loader.
func (l *Loader) EventsLoader() *EventsLoader {
	return l.eventsLoader
}

// GetOutcome returns outcome statistics for a specific sim_id.
func (l *Loader) GetOutcome(mode string, simID int) (*OutcomeStats, error) {
	table, err := l.GetMode(mode)
	if err != nil {
		return nil, err
	}

	totalWeight := table.TotalWeight()
	for _, o := range table.Outcomes {
		if o.SimID == simID {
			prob := float64(o.Weight) / float64(totalWeight)
			return &OutcomeStats{
				SimID:       o.SimID,
				Weight:      o.Weight,
				Payout:      float64(o.Payout) / 100.0,
				Probability: prob,
				Odds:        FormatOdds(prob),
			}, nil
		}
	}

	return nil, fmt.Errorf("outcome with sim_id %d not found in mode %q", simID, mode)
}

// BaseDir returns the base directory for data files.
func (l *Loader) BaseDir() string {
	return l.baseDir
}

// Reload re-reads the index.json and all LUT CSV files from disk.
// This clears all loaded events and resets the loader state.
func (l *Loader) Reload() error {
	// Clear events cache first
	l.eventsLoader.ClearAll()

	// Clear distribution cache
	l.distributionCache.InvalidateAll()

	// Clear tables
	l.tables = make(map[string]*stakergs.LookupTable)

	// Reload index and tables
	return l.Load()
}

// SaveWeights saves new weights for a specific mode to the CSV file.
// The weights must match the number of outcomes in the mode.
// This preserves the original sim_id and payout values, only updating weights.
func (l *Loader) SaveWeights(mode string, weights []uint64) error {
	// Get current table to verify structure
	table, err := l.GetMode(mode)
	if err != nil {
		return fmt.Errorf("mode %q not found: %w", mode, err)
	}

	if len(weights) != len(table.Outcomes) {
		return fmt.Errorf("weight count mismatch: got %d, expected %d", len(weights), len(table.Outcomes))
	}

	// Get the mode config to find the CSV path
	config, err := l.GetModeConfig(mode)
	if err != nil {
		return fmt.Errorf("mode config not found: %w", err)
	}

	csvPath := filepath.Join(l.baseDir, config.Weights)

	// Create temp file in same directory for atomic write
	tmpPath := csvPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	writer := bufio.NewWriter(file)

	// Write each outcome with new weight
	for i, outcome := range table.Outcomes {
		line := fmt.Sprintf("%d,%d,%d\n", outcome.SimID, weights[i], outcome.Payout)
		if _, err := writer.WriteString(line); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write line %d: %w", i, err)
		}
	}

	if err := writer.Flush(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to flush: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, csvPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename: %w", err)
	}

	// Update in-memory table
	for i := range table.Outcomes {
		table.Outcomes[i].Weight = weights[i]
	}

	// Invalidate distribution cache for this mode
	l.distributionCache.Invalidate(mode)

	return nil
}

// SaveWeightsWithBackup saves new weights and creates a backup of the original file.
// Returns the path to the backup file.
func (l *Loader) SaveWeightsWithBackup(mode string, weights []uint64) (string, error) {
	// Get the mode config to find the CSV path
	config, err := l.GetModeConfig(mode)
	if err != nil {
		return "", fmt.Errorf("mode config not found: %w", err)
	}

	csvPath := filepath.Join(l.baseDir, config.Weights)

	// Create backup with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupPath := csvPath + "." + timestamp + ".bak"

	// Copy original to backup
	originalData, err := os.ReadFile(csvPath)
	if err != nil {
		return "", fmt.Errorf("failed to read original file: %w", err)
	}

	if err := os.WriteFile(backupPath, originalData, 0644); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	// Now save the new weights
	if err := l.SaveWeights(mode, weights); err != nil {
		return backupPath, fmt.Errorf("failed to save weights (backup at %s): %w", backupPath, err)
	}

	return backupPath, nil
}
