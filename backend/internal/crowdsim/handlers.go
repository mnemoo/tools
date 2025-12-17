package crowdsim

import (
	"encoding/json"
	"net/http"

	"lutexplorer/internal/common"
	"lutexplorer/internal/lut"
	"lutexplorer/internal/ws"
)

// Handlers provides HTTP handlers for CrowdSim API.
type Handlers struct {
	loader *lut.Loader
	hub    *ws.Hub
}

// NewHandlers creates new CrowdSim handlers.
func NewHandlers(loader *lut.Loader, hub *ws.Hub) *Handlers {
	return &Handlers{
		loader: loader,
		hub:    hub,
	}
}

// HandleSimulate runs a CrowdSim simulation for a single mode.
// POST /api/crowdsim/{mode}/simulate
func (h *Handlers) HandleSimulate(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := h.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	// Parse config from request body
	var config SimConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		// io.EOF means empty body - use defaults
		// Other errors indicate invalid JSON
		if err.Error() != "EOF" {
			common.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		config = DefaultConfig()
	}

	// Validate and apply defaults
	if err := config.Validate(); err != nil {
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Create simulator
	simulator := NewCrowdSimulator(table, config)

	// Run simulation with progress reporting via WebSocket
	var result *SimResult
	if config.ParallelWorkers > 1 {
		result = simulator.RunParallel(func(p Progress) {
			if h.hub != nil {
				h.hub.Broadcast(ws.Message{
					Type: ws.MessageType("crowdsim_progress"),
					Mode: mode,
					Payload: map[string]interface{}{
						"players_complete": p.PlayersComplete,
						"total_players":    p.TotalPlayers,
						"percent_complete": p.PercentComplete,
						"elapsed_ms":       p.ElapsedMs,
					},
				})
			}
		})
	} else {
		result = simulator.Run(func(p Progress) {
			if h.hub != nil {
				h.hub.Broadcast(ws.Message{
					Type: ws.MessageType("crowdsim_progress"),
					Mode: mode,
					Payload: map[string]interface{}{
						"players_complete": p.PlayersComplete,
						"total_players":    p.TotalPlayers,
						"percent_complete": p.PercentComplete,
						"elapsed_ms":       p.ElapsedMs,
					},
				})
			}
		})
	}

	// Notify completion
	if h.hub != nil {
		h.hub.Broadcast(ws.Message{
			Type: ws.MessageType("crowdsim_complete"),
			Mode: mode,
			Payload: map[string]interface{}{
				"duration_ms": result.DurationMs,
			},
		})
	}

	common.WriteSuccess(w, result)
}

// HandleCompare runs CrowdSim for multiple modes and ranks them.
// POST /api/crowdsim/compare
func (h *Handlers) HandleCompare(w http.ResponseWriter, r *http.Request) {
	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.Modes) == 0 {
		common.WriteError(w, http.StatusBadRequest, "at least one mode required")
		return
	}

	// Validate config
	if err := req.Config.Validate(); err != nil {
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	results := make([]SimResult, 0, len(req.Modes))

	for _, mode := range req.Modes {
		table, err := h.loader.GetMode(mode)
		if err != nil {
			continue // Skip invalid modes
		}

		simulator := NewCrowdSimulator(table, req.Config)

		var result *SimResult
		if req.Config.ParallelWorkers > 1 {
			result = simulator.RunParallel(nil)
		} else {
			result = simulator.Run(nil)
		}

		results = append(results, *result)
	}

	if len(results) == 0 {
		common.WriteError(w, http.StatusNotFound, "no valid modes found")
		return
	}

	// Rank results
	ranking := RankResults(results)

	common.WriteSuccess(w, CompareResult{
		Results: results,
		Ranking: ranking,
	})
}

// HandlePresets returns available preset configurations.
// GET /api/crowdsim/presets
func (h *Handlers) HandlePresets(w http.ResponseWriter, r *http.Request) {
	presets := GetPresets()
	common.WriteSuccess(w, presets)
}

// HandleValidate validates a simulation result against theoretical RTP.
// POST /api/crowdsim/{mode}/validate
func (h *Handlers) HandleValidate(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := h.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	// Parse config
	var config SimConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Run simulation
	simulator := NewCrowdSimulator(table, config)
	var result *SimResult
	if config.ParallelWorkers > 1 {
		result = simulator.RunParallel(nil)
	} else {
		result = simulator.Run(nil)
	}

	// Validate RTP (2% tolerance)
	validation := ValidateRTP(result, 2.0)

	common.WriteSuccess(w, map[string]interface{}{
		"result":     result,
		"validation": validation,
	})
}

// HandleVolatilityCheck checks if result meets volatility profile criteria.
// POST /api/crowdsim/{mode}/volatility-check
func (h *Handlers) HandleVolatilityCheck(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := h.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	// Parse request
	var req struct {
		Config  SimConfig         `json:"config"`
		Profile VolatilityProfile `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Config = DefaultConfig()
		req.Profile = VolatilityMedium
	}

	if err := req.Config.Validate(); err != nil {
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Run simulation
	simulator := NewCrowdSimulator(table, req.Config)
	var result *SimResult
	if req.Config.ParallelWorkers > 1 {
		result = simulator.RunParallel(nil)
	} else {
		result = simulator.Run(nil)
	}

	// Check compliance
	checks := CheckVolatilityCompliance(result, req.Profile, req.Config.InitialBalance)
	thresholds := GetVolatilityThresholds()[req.Profile]

	// Count passed checks
	passed := 0
	for _, v := range checks {
		if v {
			passed++
		}
	}

	common.WriteSuccess(w, map[string]interface{}{
		"result":         result,
		"target_profile": req.Profile,
		"thresholds":     thresholds,
		"checks":         checks,
		"passed":         passed,
		"total":          len(checks),
		"compliant":      passed == len(checks),
	})
}
