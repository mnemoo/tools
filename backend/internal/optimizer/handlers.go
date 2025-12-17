package optimizer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"lutexplorer/internal/common"
	"lutexplorer/internal/lut"
	"lutexplorer/internal/ws"
)

// Handlers provides HTTP handlers for the optimizer API
type Handlers struct {
	loader *lut.Loader
	wsHub  *ws.Hub
}

// NewHandlers creates new optimizer HTTP handlers
func NewHandlers(loader *lut.Loader, wsHub *ws.Hub) *Handlers {
	return &Handlers{
		loader: loader,
		wsHub:  wsHub,
	}
}

// ============================================================================
// Apply Endpoint
// ============================================================================

// HandleApply applies weights to the LUT file
// POST /api/optimizer/{mode}/apply
func (h *Handlers) HandleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	mode := extractMode(r.URL.Path, "apply")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode required")
		return
	}

	var req struct {
		Weights      []uint64 `json:"weights"`
		CreateBackup bool     `json:"create_backup"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Weights) == 0 {
		common.WriteError(w, http.StatusBadRequest, "weights required")
		return
	}

	var backupPath string
	var err error

	if req.CreateBackup {
		backupPath, err = h.loader.SaveWeightsWithBackup(mode, req.Weights)
	} else {
		err = h.loader.SaveWeights(mode, req.Weights)
	}

	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"saved":   true,
		"message": "Weights applied successfully",
	}
	if backupPath != "" {
		response["backup_path"] = backupPath
	}

	common.WriteSuccess(w, response)
}

// ============================================================================
// Backup Endpoints
// ============================================================================

// HandleBackups lists available backups for a mode
// GET /api/optimizer/{mode}/backups
func (h *Handlers) HandleBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	mode := extractMode(r.URL.Path, "backups")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode required")
		return
	}

	config, err := h.loader.GetModeConfig(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, fmt.Sprintf("mode not found: %s", mode))
		return
	}

	baseDir := h.loader.BaseDir()
	pattern := config.Weights + ".*.bak"

	matches, err := filepath.Glob(filepath.Join(baseDir, pattern))
	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type BackupInfo struct {
		Filename  string `json:"filename"`
		Timestamp string `json:"timestamp"`
		Path      string `json:"path"`
	}

	backups := make([]BackupInfo, 0, len(matches))
	for _, match := range matches {
		filename := filepath.Base(match)
		parts := strings.Split(filename, ".")
		timestamp := ""
		if len(parts) >= 3 {
			timestamp = parts[len(parts)-2]
		}

		backups = append(backups, BackupInfo{
			Filename:  filename,
			Timestamp: timestamp,
			Path:      match,
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp > backups[j].Timestamp
	})

	common.WriteSuccess(w, backups)
}

// HandleRestore restores weights from a backup file
// POST /api/optimizer/{mode}/restore
func (h *Handlers) HandleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	mode := extractMode(r.URL.Path, "restore")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode required")
		return
	}

	var req struct {
		BackupFile   string `json:"backup_file"`
		CreateBackup bool   `json:"create_backup"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.BackupFile == "" {
		common.WriteError(w, http.StatusBadRequest, "backup_file required")
		return
	}

	backupPath := req.BackupFile
	if !filepath.IsAbs(backupPath) {
		backupPath = filepath.Join(h.loader.BaseDir(), req.BackupFile)
	}

	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, fmt.Sprintf("backup file not found: %s", err.Error()))
		return
	}

	weights, err := parseWeightsFromCSV(backupData)
	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to parse backup: %s", err.Error()))
		return
	}

	var preRestoreBackup string
	if req.CreateBackup {
		preRestoreBackup, err = h.loader.SaveWeightsWithBackup(mode, weights)
		if err != nil {
			common.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create pre-restore backup: %s", err.Error()))
			return
		}
	} else {
		if err := h.loader.SaveWeights(mode, weights); err != nil {
			common.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	response := map[string]interface{}{
		"restored":      true,
		"restored_from": req.BackupFile,
		"message":       "Weights restored successfully",
	}
	if preRestoreBackup != "" {
		response["pre_restore_backup"] = preRestoreBackup
	}

	common.WriteSuccess(w, response)
}

// ============================================================================
// Utilities
// ============================================================================

func parseWeightsFromCSV(data []byte) ([]uint64, error) {
	var weights []uint64
	lines := strings.Split(string(data), "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil, fmt.Errorf("line %d: expected 3 fields, got %d", lineNum+1, len(parts))
		}

		weight, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid weight: %w", lineNum+1, err)
		}

		weights = append(weights, weight)
	}

	return weights, nil
}

func extractMode(path, action string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	optimizerIdx := -1
	for i, p := range parts {
		if p == "optimizer" {
			optimizerIdx = i
			break
		}
	}

	if optimizerIdx < 0 || optimizerIdx+1 >= len(parts) {
		return ""
	}

	mode := parts[optimizerIdx+1]

	if mode == action || mode == "bucket-presets" || mode == "profiles" || mode == "generate-configs" || mode == "generate-config" {
		return ""
	}

	return mode
}

// getModeNote returns a helpful note about the mode type
func getModeNote(cost float64) string {
	if cost > 1.5 {
		return fmt.Sprintf("Bonus mode (cost=%.0fx). Payouts are normalized: a %.0fx absolute payout = 1.0x normalized.", cost, cost)
	}
	return "Standard mode. Payouts are shown as bet multipliers."
}

// ============================================================================
// Bucket Optimizer Endpoints
// ============================================================================

// BucketOptimizeRequest is the API request for bucket-based optimization
type BucketOptimizeRequest struct {
	TargetRTP    float64        `json:"target_rtp"`    // Target RTP (e.g., 0.97)
	RTPTolerance float64        `json:"rtp_tolerance"` // Tolerance (e.g., 0.001)
	Buckets      []BucketConfig `json:"buckets"`       // Payout range configurations
	SaveToFile   bool           `json:"save_to_file"`  // Save optimized weights to LUT file
	CreateBackup bool           `json:"create_backup"` // Create backup before saving
}

// HandleBucketOptimize runs bucket-based optimization on a mode
// POST /api/optimizer/{mode}/bucket-optimize
func (h *Handlers) HandleBucketOptimize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	mode := extractMode(r.URL.Path, "bucket-optimize")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode required")
		return
	}

	// Parse request
	var req BucketOptimizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %s", err.Error()))
		return
	}

	// Apply defaults
	if req.TargetRTP <= 0 {
		req.TargetRTP = 0.97
	}
	if req.RTPTolerance <= 0 {
		req.RTPTolerance = 0.001
	}

	// Validate buckets if provided
	if len(req.Buckets) > 0 {
		if err := ValidateBuckets(req.Buckets); err != nil {
			common.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid buckets: %s", err.Error()))
			return
		}
	}

	// Load table
	table, err := h.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, fmt.Sprintf("mode not found: %s", mode))
		return
	}

	// If no buckets provided, suggest them based on table
	buckets := req.Buckets
	if len(buckets) == 0 {
		buckets = SuggestBuckets(table, req.TargetRTP)
	}

	// Create optimizer config
	config := &BucketOptimizerConfig{
		TargetRTP:    req.TargetRTP,
		RTPTolerance: req.RTPTolerance,
		Buckets:      buckets,
		MinWeight:    1,
	}
	optimizer := NewBucketOptimizer(config)

	// Run optimization
	result, err := optimizer.OptimizeTable(table)
	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Save if requested
	var saveInfo map[string]interface{}
	if req.SaveToFile && result.NewWeights != nil {
		if req.CreateBackup {
			backupPath, err := h.loader.SaveWeightsWithBackup(mode, result.NewWeights)
			if err != nil {
				common.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("save failed: %s", err.Error()))
				return
			}
			saveInfo = map[string]interface{}{
				"saved":       true,
				"backup_path": backupPath,
			}
		} else {
			if err := h.loader.SaveWeights(mode, result.NewWeights); err != nil {
				common.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("save failed: %s", err.Error()))
				return
			}
			saveInfo = map[string]interface{}{"saved": true}
		}
	}

	// Get mode cost for context
	cost := table.Cost
	if cost <= 0 {
		cost = 1.0
	}
	isBonusMode := cost > 1.5

	// Build response
	response := map[string]interface{}{
		"original_rtp":    result.OriginalRTP,
		"final_rtp":       result.FinalRTP,
		"target_rtp":      result.TargetRTP,
		"converged":       result.Converged,
		"total_weight":    result.TotalWeight,
		"bucket_results":  result.BucketResults,
		"loss_result":     result.LossResult,
		"warnings":        result.Warnings,
		"outcome_details": result.OutcomeDetails,
		"mode_info": map[string]interface{}{
			"cost":          cost,
			"is_bonus_mode": isBonusMode,
			"note":          getModeNote(cost),
		},
		"config": map[string]interface{}{
			"target_rtp": req.TargetRTP,
			"buckets":    buckets,
		},
	}

	if saveInfo != nil {
		response["save_result"] = saveInfo
	}

	common.WriteSuccess(w, response)
}

// HandleBucketPresets returns available bucket presets
// GET /api/optimizer/bucket-presets
func (h *Handlers) HandleBucketPresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	// Return several preset configurations
	presets := map[string]interface{}{
		"default": DefaultBucketConfig().Buckets,
		"conservative": []BucketConfig{
			{Name: "sub_1x", MinPayout: 0.01, MaxPayout: 1, Type: ConstraintFrequency, Frequency: 2.5},
			{Name: "small", MinPayout: 1, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 4},
			{Name: "medium", MinPayout: 5, MaxPayout: 20, Type: ConstraintFrequency, Frequency: 15},
			{Name: "large", MinPayout: 20, MaxPayout: 100, Type: ConstraintFrequency, Frequency: 80},
			{Name: "huge", MinPayout: 100, MaxPayout: 1000, Type: ConstraintRTPPercent, RTPPercent: 8},
			{Name: "jackpot", MinPayout: 1000, MaxPayout: 100000, Type: ConstraintRTPPercent, RTPPercent: 1},
		},
		"aggressive": []BucketConfig{
			{Name: "sub_1x", MinPayout: 0.01, MaxPayout: 1, Type: ConstraintFrequency, Frequency: 5},
			{Name: "small", MinPayout: 1, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 10},
			{Name: "medium", MinPayout: 5, MaxPayout: 20, Type: ConstraintFrequency, Frequency: 50},
			{Name: "large", MinPayout: 20, MaxPayout: 100, Type: ConstraintFrequency, Frequency: 200},
			{Name: "huge", MinPayout: 100, MaxPayout: 1000, Type: ConstraintRTPPercent, RTPPercent: 3},
			{Name: "jackpot", MinPayout: 1000, MaxPayout: 100000, Type: ConstraintRTPPercent, RTPPercent: 0.3},
		},
	}

	common.WriteSuccess(w, presets)
}

// HandleSuggestBuckets analyzes a mode and suggests bucket configuration
// GET /api/optimizer/{mode}/suggest-buckets
func (h *Handlers) HandleSuggestBuckets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	mode := extractMode(r.URL.Path, "suggest-buckets")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode required")
		return
	}

	// Load table
	table, err := h.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, fmt.Sprintf("mode not found: %s", mode))
		return
	}

	// Get target RTP from query param or use default
	targetRTP := 0.97
	if rtpStr := r.URL.Query().Get("target_rtp"); rtpStr != "" {
		if parsed, err := strconv.ParseFloat(rtpStr, 64); err == nil && parsed > 0 && parsed < 1 {
			targetRTP = parsed
		}
	}

	// Suggest buckets
	buckets := SuggestBuckets(table, targetRTP)

	// Also return some table stats
	cost := table.Cost
	if cost <= 0 {
		cost = 1.0
	}
	var maxPayout, minPayout float64
	minPayout = 999999
	payoutCounts := make(map[string]int)

	for _, outcome := range table.Outcomes {
		payout := float64(outcome.Payout) / 100.0 / cost
		if payout > maxPayout {
			maxPayout = payout
		}
		if payout > 0 && payout < minPayout {
			minPayout = payout
		}
		// Categorize
		switch {
		case payout <= 0:
			payoutCounts["loss"]++
		case payout < 1:
			payoutCounts["sub_1x"]++
		case payout < 5:
			payoutCounts["1x-5x"]++
		case payout < 20:
			payoutCounts["5x-20x"]++
		case payout < 100:
			payoutCounts["20x-100x"]++
		case payout < 1000:
			payoutCounts["100x-1000x"]++
		default:
			payoutCounts["1000x+"]++
		}
	}

	isBonusMode := cost > 1.5

	common.WriteSuccess(w, map[string]interface{}{
		"suggested_buckets": buckets,
		"table_stats": map[string]interface{}{
			"outcome_count":  len(table.Outcomes),
			"max_payout":     maxPayout,
			"min_payout":     minPayout,
			"payout_counts":  payoutCounts,
			"current_rtp":    table.RTP(),
		},
		"mode_info": map[string]interface{}{
			"cost":          cost,
			"is_bonus_mode": isBonusMode,
			"note":          getModeNote(cost),
		},
	})
}

// ============================================================================
// Config Generator Endpoints
// ============================================================================

// GenerateConfigRequest is the API request for config generation
type GenerateConfigRequest struct {
	TargetRTP float64       `json:"target_rtp"` // e.g., 0.96
	MaxWin    float64       `json:"max_win"`    // e.g., 5000
	Profile   PlayerProfile `json:"profile"`    // Optional: specific profile
}

// HandleGenerateConfigs generates bucket configs for all profiles
// GET /api/optimizer/generate-configs?target_rtp=0.96&max_win=5000
func (h *Handlers) HandleGenerateConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	// Parse query params
	targetRTP := 0.96
	if rtpStr := r.URL.Query().Get("target_rtp"); rtpStr != "" {
		if parsed, err := strconv.ParseFloat(rtpStr, 64); err == nil && parsed > 0 && parsed <= 1 {
			targetRTP = parsed
		}
	}

	maxWin := 5000.0
	if maxWinStr := r.URL.Query().Get("max_win"); maxWinStr != "" {
		if parsed, err := strconv.ParseFloat(maxWinStr, 64); err == nil && parsed > 0 {
			maxWin = parsed
		}
	}

	generator := NewConfigGenerator()
	response := generator.GenerateAllProfiles(targetRTP, maxWin)

	common.WriteSuccess(w, response)
}

// HandleGenerateConfig generates bucket config for a specific profile
// POST /api/optimizer/generate-config
func (h *Handlers) HandleGenerateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req GenerateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %s", err.Error()))
		return
	}

	// Apply defaults
	if req.TargetRTP <= 0 || req.TargetRTP > 1 {
		req.TargetRTP = 0.96
	}
	if req.MaxWin <= 0 {
		req.MaxWin = 5000
	}
	if req.Profile == "" {
		req.Profile = ProfileMediumVol
	}

	generator := NewConfigGenerator()
	config := generator.GenerateConfig(req.TargetRTP, req.MaxWin, req.Profile)

	// Validate the generated config
	if err := ValidateGeneratedConfig(config); err != nil {
		common.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("generated config invalid: %s", err.Error()))
		return
	}

	common.WriteSuccess(w, config)
}

// HandleGenerateConfigsForMode generates configs based on a mode's actual max payout
// GET /api/optimizer/{mode}/generate-configs?target_rtp=0.96
func (h *Handlers) HandleGenerateConfigsForMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	mode := extractMode(r.URL.Path, "generate-configs")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode required")
		return
	}

	// Load table to get actual max payout
	table, err := h.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, fmt.Sprintf("mode not found: %s", mode))
		return
	}

	// Calculate max payout from table
	cost := table.Cost
	if cost <= 0 {
		cost = 1.0
	}
	var maxPayout float64
	for _, outcome := range table.Outcomes {
		payout := float64(outcome.Payout) / 100.0 / cost
		if payout > maxPayout {
			maxPayout = payout
		}
	}

	// Parse target RTP from query
	targetRTP := 0.96
	if rtpStr := r.URL.Query().Get("target_rtp"); rtpStr != "" {
		if parsed, err := strconv.ParseFloat(rtpStr, 64); err == nil && parsed > 0 && parsed <= 1 {
			targetRTP = parsed
		}
	}

	generator := NewConfigGenerator()
	response := generator.GenerateAllProfiles(targetRTP, maxPayout)

	// Add mode-specific info
	common.WriteSuccess(w, map[string]interface{}{
		"mode":        mode,
		"max_payout":  maxPayout,
		"target_rtp":  targetRTP,
		"current_rtp": table.RTP(),
		"configs":     response.Configs,
	})
}

// HandleProfiles returns available player profiles
// GET /api/optimizer/profiles
func (h *Handlers) HandleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	profiles := []map[string]interface{}{
		{
			"id":          ProfileLowVol,
			"name":        "Low Volatility",
			"description": ProfileDescriptions[ProfileLowVol],
		},
		{
			"id":          ProfileMediumVol,
			"name":        "Medium Volatility",
			"description": ProfileDescriptions[ProfileMediumVol],
		},
		{
			"id":          ProfileHighVol,
			"name":        "High Volatility",
			"description": ProfileDescriptions[ProfileHighVol],
		},
	}

	common.WriteSuccess(w, profiles)
}

// RegisterRoutes registers all optimizer routes
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/optimizer/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		// General endpoints
		case strings.HasSuffix(path, "/apply"):
			h.HandleApply(w, r)
		case strings.HasSuffix(path, "/backups"):
			h.HandleBackups(w, r)
		case strings.HasSuffix(path, "/restore"):
			h.HandleRestore(w, r)

		// Bucket optimizer endpoints
		case strings.HasSuffix(path, "/bucket-optimize"):
			h.HandleBucketOptimize(w, r)
		case strings.HasSuffix(path, "/suggest-buckets"):
			h.HandleSuggestBuckets(w, r)
		case path == "/api/optimizer/bucket-presets":
			h.HandleBucketPresets(w, r)

		// Config generator endpoints
		case path == "/api/optimizer/generate-configs":
			h.HandleGenerateConfigs(w, r)
		case path == "/api/optimizer/generate-config":
			h.HandleGenerateConfig(w, r)
		case path == "/api/optimizer/profiles":
			h.HandleProfiles(w, r)
		case strings.HasSuffix(path, "/generate-configs"):
			h.HandleGenerateConfigsForMode(w, r)

		default:
			common.WriteError(w, http.StatusNotFound, "endpoint not found")
		}
	})
}
