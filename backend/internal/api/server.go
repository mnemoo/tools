// Package api provides the HTTP API server for LUT Explorer.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"lutexplorer/internal/bgloader"
	"lutexplorer/internal/common"
	"lutexplorer/internal/crowdsim"
	"lutexplorer/internal/lgs"
	"lutexplorer/internal/lut"
	"lutexplorer/internal/optimizer"
	"lutexplorer/internal/ws"

	"github.com/rs/cors"
	"stakergs"
)

// Server is the HTTP API server.
type Server struct {
	loader            *lut.Loader
	addr              string
	lgsHandlers       *lgs.Handlers
	lgsSessions       *lgs.SessionManager
	crowdsimHandlers  *crowdsim.Handlers
	optimizerHandlers *optimizer.Handlers
	wsHub             *ws.Hub
	bgLoader          *bgloader.BackgroundLoader
}

// NewServer creates a new API server.
func NewServer(loader *lut.Loader, addr string, hub *ws.Hub) *Server {
	sessions := lgs.NewSessionManager()
	return &Server{
		loader:            loader,
		addr:              addr,
		lgsSessions:       sessions,
		lgsHandlers:       lgs.NewHandlers(loader, sessions, hub),
		crowdsimHandlers:  crowdsim.NewHandlers(loader, hub),
		optimizerHandlers: optimizer.NewHandlers(loader, hub),
		wsHub:             hub,
	}
}

// SetBackgroundLoader sets the background loader for the server.
func (s *Server) SetBackgroundLoader(bl *bgloader.BackgroundLoader) {
	s.bgLoader = bl
}

// Hub returns the WebSocket hub.
func (s *Server) Hub() *ws.Hub {
	return s.wsHub
}

// IndexInfo contains basic information about the loaded index.
type IndexInfo struct {
	Modes []lut.ModeSummary `json:"modes"`
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/index", s.handleIndex)
	mux.HandleFunc("GET /api/modes", s.handleModes)
	mux.HandleFunc("GET /api/mode/{mode}", s.handleMode)
	mux.HandleFunc("GET /api/mode/{mode}/stats", s.handleModeStats)
	mux.HandleFunc("GET /api/mode/{mode}/distribution", s.handleModeDistribution)
	mux.HandleFunc("GET /api/mode/{mode}/outcomes", s.handleModeOutcomes)
	mux.HandleFunc("GET /api/compare", s.handleCompare)

	// Events API
	mux.HandleFunc("POST /api/mode/{mode}/events/load", s.handleLoadEvents)
	mux.HandleFunc("GET /api/mode/{mode}/event/{simID}", s.handleGetEvent)

	// Simulator API
	mux.HandleFunc("POST /api/mode/{mode}/simulate", s.handleSimulate)
	mux.HandleFunc("POST /api/mode/{mode}/simulate/quick", s.handleQuickSimulate)

	// CrowdSim API
	mux.HandleFunc("POST /api/crowdsim/{mode}/simulate", s.crowdsimHandlers.HandleSimulate)
	mux.HandleFunc("POST /api/crowdsim/compare", s.crowdsimHandlers.HandleCompare)
	mux.HandleFunc("GET /api/crowdsim/presets", s.crowdsimHandlers.HandlePresets)
	mux.HandleFunc("POST /api/crowdsim/{mode}/validate", s.crowdsimHandlers.HandleValidate)
	mux.HandleFunc("POST /api/crowdsim/{mode}/volatility-check", s.crowdsimHandlers.HandleVolatilityCheck)

	// Optimizer API
	s.optimizerHandlers.RegisterRoutes(mux)

	// LGS (Local Game Server) - RGS-compatible endpoints
	// Wallet endpoints
	mux.HandleFunc("POST /wallet/authenticate", s.lgsHandlers.Authenticate)
	mux.HandleFunc("POST /wallet/play", s.lgsHandlers.Play)
	mux.HandleFunc("POST /wallet/end-round", s.lgsHandlers.EndRound)

	// Bet endpoints
	mux.HandleFunc("POST /bet/event", s.lgsHandlers.Event)
	mux.HandleFunc("GET /bet/replay/{game}/{version}/{mode}/{event}", s.lgsHandlers.Replay)

	// Additional LGS utility endpoints
	mux.HandleFunc("GET /lgs/health", s.lgsHandlers.Health)
	mux.HandleFunc("GET /lgs/sessions", s.lgsHandlers.Sessions)
	mux.HandleFunc("POST /lgs/batchplay", s.lgsHandlers.BatchPlay)
	mux.HandleFunc("POST /lgs/history", s.lgsHandlers.History)
	mux.HandleFunc("DELETE /lgs/history", s.lgsHandlers.ClearHistory)
	mux.HandleFunc("GET /lgs/stats", s.lgsHandlers.Stats)
	mux.HandleFunc("DELETE /lgs/stats", s.lgsHandlers.ClearStats)
	mux.HandleFunc("POST /lgs/reset-balance", s.lgsHandlers.ResetBalance)
	mux.HandleFunc("POST /lgs/set-balance", s.lgsHandlers.SetBalance)
	mux.HandleFunc("POST /lgs/force-outcome", s.lgsHandlers.ForceOutcome)
	mux.HandleFunc("GET /lgs/force-outcome", s.lgsHandlers.GetForcedOutcomes)
	mux.HandleFunc("DELETE /lgs/force-outcome", s.lgsHandlers.ClearForcedOutcome)
	mux.HandleFunc("POST /lgs/rtp-bias", s.lgsHandlers.SetRTPBias)
	mux.HandleFunc("GET /lgs/rtp-bias", s.lgsHandlers.GetRTPBias)

	// WebSocket endpoint
	mux.HandleFunc("GET /ws", s.wsHub.ServeWs)

	// Compliance API
	mux.HandleFunc("GET /api/mode/{mode}/compliance", s.handleModeCompliance)
	mux.HandleFunc("GET /api/compliance", s.handleAllCompliance)

	// Background loader API
	mux.HandleFunc("GET /api/loader/status", s.handleLoaderStatus)
	mux.HandleFunc("POST /api/loader/boost", s.handleLoaderBoost)
	mux.HandleFunc("DELETE /api/loader/boost", s.handleLoaderUnboost)
	mux.HandleFunc("GET /api/loader/priority", s.handleLoaderPriority)
	mux.HandleFunc("POST /api/reload", s.handleReload)

	// CORS middleware
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	// Logging middleware
	loggingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log all requests except WebSocket upgrades and high-frequency endpoints
		if r.URL.Path != "/ws" && r.URL.Path != "/api/loader/status" {
			log.Printf("[HTTP] %s %s", r.Method, r.URL.Path)
		}
		c.Handler(mux).ServeHTTP(w, r)
	})

	log.Printf("Starting LUT Explorer API server on %s", s.addr)
	log.Printf("LGS endpoints available at /wallet/authenticate, /wallet/play, /wallet/end-round")
	return http.ListenAndServe(s.addr, loggingHandler)
}

// GetHandler returns the HTTP handler for use with custom servers (e.g., HTTPS).
func (s *Server) GetHandler() http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/index", s.handleIndex)
	mux.HandleFunc("GET /api/modes", s.handleModes)
	mux.HandleFunc("GET /api/mode/{mode}", s.handleMode)
	mux.HandleFunc("GET /api/mode/{mode}/stats", s.handleModeStats)
	mux.HandleFunc("GET /api/mode/{mode}/distribution", s.handleModeDistribution)
	mux.HandleFunc("GET /api/mode/{mode}/outcomes", s.handleModeOutcomes)
	mux.HandleFunc("GET /api/compare", s.handleCompare)

	// Events API
	mux.HandleFunc("POST /api/mode/{mode}/events/load", s.handleLoadEvents)
	mux.HandleFunc("GET /api/mode/{mode}/event/{simID}", s.handleGetEvent)

	// Simulator API
	mux.HandleFunc("POST /api/mode/{mode}/simulate", s.handleSimulate)
	mux.HandleFunc("POST /api/mode/{mode}/simulate/quick", s.handleQuickSimulate)

	// CrowdSim API
	mux.HandleFunc("POST /api/crowdsim/{mode}/simulate", s.crowdsimHandlers.HandleSimulate)
	mux.HandleFunc("POST /api/crowdsim/compare", s.crowdsimHandlers.HandleCompare)
	mux.HandleFunc("GET /api/crowdsim/presets", s.crowdsimHandlers.HandlePresets)
	mux.HandleFunc("POST /api/crowdsim/{mode}/validate", s.crowdsimHandlers.HandleValidate)
	mux.HandleFunc("POST /api/crowdsim/{mode}/volatility-check", s.crowdsimHandlers.HandleVolatilityCheck)

	// Optimizer API
	s.optimizerHandlers.RegisterRoutes(mux)

	// LGS (Local Game Server) - RGS-compatible endpoints
	mux.HandleFunc("POST /wallet/authenticate", s.lgsHandlers.Authenticate)
	mux.HandleFunc("POST /wallet/play", s.lgsHandlers.Play)
	mux.HandleFunc("POST /wallet/end-round", s.lgsHandlers.EndRound)

	// Bet endpoints
	mux.HandleFunc("POST /bet/event", s.lgsHandlers.Event)
	mux.HandleFunc("GET /bet/replay/{game}/{version}/{mode}/{event}", s.lgsHandlers.Replay)

	// Additional LGS utility endpoints
	mux.HandleFunc("GET /lgs/health", s.lgsHandlers.Health)
	mux.HandleFunc("GET /lgs/sessions", s.lgsHandlers.Sessions)
	mux.HandleFunc("POST /lgs/batchplay", s.lgsHandlers.BatchPlay)
	mux.HandleFunc("POST /lgs/history", s.lgsHandlers.History)
	mux.HandleFunc("DELETE /lgs/history", s.lgsHandlers.ClearHistory)
	mux.HandleFunc("GET /lgs/stats", s.lgsHandlers.Stats)
	mux.HandleFunc("DELETE /lgs/stats", s.lgsHandlers.ClearStats)
	mux.HandleFunc("POST /lgs/reset-balance", s.lgsHandlers.ResetBalance)
	mux.HandleFunc("POST /lgs/set-balance", s.lgsHandlers.SetBalance)
	mux.HandleFunc("POST /lgs/force-outcome", s.lgsHandlers.ForceOutcome)
	mux.HandleFunc("GET /lgs/force-outcome", s.lgsHandlers.GetForcedOutcomes)
	mux.HandleFunc("DELETE /lgs/force-outcome", s.lgsHandlers.ClearForcedOutcome)
	mux.HandleFunc("POST /lgs/rtp-bias", s.lgsHandlers.SetRTPBias)
	mux.HandleFunc("GET /lgs/rtp-bias", s.lgsHandlers.GetRTPBias)

	// WebSocket endpoint
	mux.HandleFunc("GET /ws", s.wsHub.ServeWs)

	// Compliance API
	mux.HandleFunc("GET /api/mode/{mode}/compliance", s.handleModeCompliance)
	mux.HandleFunc("GET /api/compliance", s.handleAllCompliance)

	// Background loader API
	mux.HandleFunc("GET /api/loader/status", s.handleLoaderStatus)
	mux.HandleFunc("POST /api/loader/boost", s.handleLoaderBoost)
	mux.HandleFunc("DELETE /api/loader/boost", s.handleLoaderUnboost)
	mux.HandleFunc("GET /api/loader/priority", s.handleLoaderPriority)
	mux.HandleFunc("POST /api/reload", s.handleReload)

	// CORS middleware
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	// Logging middleware
	loggingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" && r.URL.Path != "/api/loader/status" {
			log.Printf("[HTTP] %s %s", r.Method, r.URL.Path)
		}
		c.Handler(mux).ServeHTTP(w, r)
	})

	return loggingHandler
}


func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	common.WriteSuccess(w, map[string]string{"status": "ok"})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	index := s.loader.GetIndex()
	if index == nil {
		common.WriteError(w, http.StatusInternalServerError, "index not loaded")
		return
	}

	info := IndexInfo{
		Modes: s.loader.GetModeSummaries(),
	}

	common.WriteSuccess(w, info)
}

func (s *Server) handleModes(w http.ResponseWriter, r *http.Request) {
	summaries := s.loader.GetModeSummaries()
	common.WriteSuccess(w, summaries)
}

func (s *Server) handleMode(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := s.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	summary := lut.ModeSummary{
		Mode:      table.Mode,
		Outcomes:  len(table.Outcomes),
		RTP:       table.RTP(),
		HitRate:   table.HitRate(),
		MaxPayout: float64(table.MaxPayout()) / 100.0,
	}

	common.WriteSuccess(w, summary)
}

func (s *Server) handleModeStats(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := s.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	stats := s.loader.Analyzer().Analyze(table)
	common.WriteSuccess(w, stats)
}

func (s *Server) handleModeDistribution(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := s.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	totalWeight := table.TotalWeight()
	distribution := s.loader.Analyzer().BuildDistribution(table, totalWeight)

	common.WriteSuccess(w, distribution)
}

func (s *Server) handleModeOutcomes(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := s.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	// Convert outcomes to response format
	type OutcomeResponse struct {
		SimID       int     `json:"sim_id"`
		Weight      uint64  `json:"weight"`
		Payout      float64 `json:"payout"`
		Probability float64 `json:"probability"`
	}

	totalWeight := table.TotalWeight()
	outcomes := make([]OutcomeResponse, len(table.Outcomes))
	for i, o := range table.Outcomes {
		outcomes[i] = OutcomeResponse{
			SimID:       o.SimID,
			Weight:      o.Weight,
			Payout:      float64(o.Payout) / 100.0,
			Probability: float64(o.Weight) / float64(totalWeight),
		}
	}

	common.WriteSuccess(w, outcomes)
}

// CompareResponse contains comparison data for multiple modes.
// FailedMode contains information about a mode that failed to load.
type FailedMode struct {
	Mode  string `json:"mode"`
	Error string `json:"error"`
}

type CompareResponse struct {
	Modes       []CompareItem `json:"modes"`
	FailedModes []FailedMode  `json:"failed_modes,omitempty"`
}

// CompareItem contains comparison data for a single mode.
type CompareItem struct {
	Mode         string  `json:"mode"`
	RTP          float64 `json:"rtp"`
	HitRate      float64 `json:"hit_rate"`
	MaxPayout    float64 `json:"max_payout"`
	Volatility   float64 `json:"volatility"`
	MeanPayout   float64 `json:"mean_payout"`
	MedianPayout float64 `json:"median_payout"`
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	modesParam := r.URL.Query()["mode"]
	if len(modesParam) == 0 {
		// Compare all modes if none specified
		modesParam = s.loader.ListModes()
	}

	items := make([]CompareItem, 0, len(modesParam))
	var failedModes []FailedMode

	for _, modeName := range modesParam {
		table, err := s.loader.GetMode(modeName)
		if err != nil {
			failedModes = append(failedModes, FailedMode{
				Mode:  modeName,
				Error: err.Error(),
			})
			continue
		}

		stats := s.loader.Analyzer().Analyze(table)
		items = append(items, CompareItem{
			Mode:         modeName,
			RTP:          stats.RTP,
			HitRate:      stats.HitRate,
			MaxPayout:    stats.MaxPayout,
			Volatility:   stats.Volatility,
			MeanPayout:   stats.MeanPayout,
			MedianPayout: stats.MedianPayout,
		})
	}

	common.WriteSuccess(w, CompareResponse{
		Modes:       items,
		FailedModes: failedModes,
	})
}

// handleLoadEvents loads events for a mode from the .jsonl.zst file.
func (s *Server) handleLoadEvents(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	// Check if already loaded
	if s.loader.EventsLoader().IsLoaded(mode) {
		common.WriteSuccess(w, map[string]interface{}{
			"mode":   mode,
			"loaded": true,
			"count":  s.loader.EventsLoader().GetEventCount(mode),
		})
		return
	}

	// Load events
	if err := s.loader.LoadEvents(mode); err != nil {
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	common.WriteSuccess(w, map[string]interface{}{
		"mode":   mode,
		"loaded": true,
		"count":  s.loader.EventsLoader().GetEventCount(mode),
	})
}

// handleGetEvent returns a specific event with its statistics.
func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	simIDStr := r.PathValue("simID")

	if mode == "" || simIDStr == "" {
		common.WriteError(w, http.StatusBadRequest, "mode and simID parameters required")
		return
	}

	simID, err := parseSimID(simIDStr)
	if err != nil {
		common.WriteError(w, http.StatusBadRequest, "invalid simID: "+err.Error())
		return
	}

	// Get outcome statistics
	outcome, err := s.loader.GetOutcome(mode, simID)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	// Check if events are loaded
	if !s.loader.EventsLoader().IsLoaded(mode) {
		// Return outcome stats without event data
		common.WriteSuccess(w, map[string]interface{}{
			"sim_id":        outcome.SimID,
			"weight":        outcome.Weight,
			"payout":        outcome.Payout,
			"probability":   outcome.Probability,
			"odds":          outcome.Odds,
			"event":         nil,
			"events_loaded": false,
		})
		return
	}

	// Get event data (may not exist even if events file is loaded)
	eventInfo, err := s.loader.EventsLoader().GetEventInfo(mode, simID, outcome)
	if err != nil {
		// Event not found in events file, return outcome stats without event
		common.WriteSuccess(w, map[string]interface{}{
			"sim_id":        outcome.SimID,
			"weight":        outcome.Weight,
			"payout":        outcome.Payout,
			"probability":   outcome.Probability,
			"odds":          outcome.Odds,
			"event":         nil,
			"events_loaded": true,
			"event_missing": true,
		})
		return
	}

	common.WriteSuccess(w, map[string]interface{}{
		"sim_id":        eventInfo.SimID,
		"weight":        eventInfo.Weight,
		"payout":        eventInfo.Payout,
		"probability":   eventInfo.Probability,
		"odds":          eventInfo.Odds,
		"event":         eventInfo.Event,
		"events_loaded": true,
	})
}

func parseSimID(s string) (int, error) {
	var simID int
	_, err := fmt.Sscanf(s, "%d", &simID)
	return simID, err
}

// SimulateRequest holds the request body for simulation.
type SimulateRequest struct {
	Spins       int       `json:"spins"`
	Trials      int       `json:"trials"`
	TargetRTP   float64   `json:"target_rtp"`
	TestSpins   []int     `json:"test_spins"`
	TestWeights []float64 `json:"test_weights"`
}

// handleSimulate runs a full simulation with multiple trials.
func (s *Server) handleSimulate(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := s.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Validate and set defaults
	if req.Spins <= 0 {
		req.Spins = common.DefaultSpins
	}
	if req.Spins > common.MaxSpins {
		req.Spins = common.MaxSpins
	}
	if req.Trials <= 0 {
		req.Trials = common.DefaultTrials
	}
	if req.Trials > common.MaxTrials {
		req.Trials = common.MaxTrials
	}
	if req.TargetRTP <= 0 {
		req.TargetRTP = 0.97
	}
	if len(req.TestSpins) == 0 {
		req.TestSpins = []int{100, 500, 1000}
	}

	// Use mode cost as bet
	bet := table.Cost
	if bet <= 0 {
		bet = 1.0
	}

	config := lut.SimulationConfig{
		Spins:       req.Spins,
		Trials:      req.Trials,
		Bet:         bet,
		TargetRTP:   req.TargetRTP,
		TestSpins:   req.TestSpins,
		TestWeights: req.TestWeights,
	}

	result := s.loader.Simulator().RunSimulation(table, config)
	common.WriteSuccess(w, result)
}

// QuickSimulateRequest holds the request body for quick simulation.
type QuickSimulateRequest struct {
	Spins int `json:"spins"`
}

// handleQuickSimulate runs a quick single-trial simulation with spin-by-spin results.
func (s *Server) handleQuickSimulate(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := s.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	var req QuickSimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Validate and set defaults
	if req.Spins <= 0 {
		req.Spins = 100
	}
	if req.Spins > 10000 {
		req.Spins = 10000
	}

	// Use mode cost as bet
	bet := table.Cost
	if bet <= 0 {
		bet = 1.0
	}

	result := s.loader.Simulator().RunQuickSimulation(table, req.Spins, bet)
	common.WriteSuccess(w, result)
}

// handleLoaderStatus returns the current status of background loading.
func (s *Server) handleLoaderStatus(w http.ResponseWriter, r *http.Request) {
	if s.bgLoader == nil {
		common.WriteError(w, http.StatusServiceUnavailable, "background loader not initialized")
		return
	}

	status := s.bgLoader.GetStatus()
	priority := s.bgLoader.GetPriority().String()

	common.WriteSuccess(w, map[string]interface{}{
		"priority":  priority,
		"modes":     status,
		"ws_clients": s.wsHub.ClientCount(),
	})
}

// handleLoaderBoost enables high priority (turbo) mode for loading.
func (s *Server) handleLoaderBoost(w http.ResponseWriter, r *http.Request) {
	if s.bgLoader == nil {
		common.WriteError(w, http.StatusServiceUnavailable, "background loader not initialized")
		return
	}

	s.bgLoader.SetPriority(bgloader.PriorityHigh)
	common.WriteSuccess(w, map[string]string{
		"priority": "high",
		"message":  "Loading priority set to HIGH - full CPU utilization",
	})
}

// handleLoaderUnboost disables high priority mode, returning to slow background loading.
func (s *Server) handleLoaderUnboost(w http.ResponseWriter, r *http.Request) {
	if s.bgLoader == nil {
		common.WriteError(w, http.StatusServiceUnavailable, "background loader not initialized")
		return
	}

	s.bgLoader.SetPriority(bgloader.PriorityLow)
	common.WriteSuccess(w, map[string]string{
		"priority": "low",
		"message":  "Loading priority set to LOW - slow background loading",
	})
}

// handleLoaderPriority returns the current loading priority.
func (s *Server) handleLoaderPriority(w http.ResponseWriter, r *http.Request) {
	if s.bgLoader == nil {
		common.WriteError(w, http.StatusServiceUnavailable, "background loader not initialized")
		return
	}

	priority := s.bgLoader.GetPriority()
	common.WriteSuccess(w, map[string]interface{}{
		"priority":    priority.String(),
		"description": getPriorityDescription(priority),
	})
}

func getPriorityDescription(p bgloader.Priority) string {
	if p == bgloader.PriorityHigh {
		return "High priority - loading at full CPU speed"
	}
	return "Low priority - slow background loading (~100 lines/sec)"
}

// handleReload reloads the index.json and all books from disk.
func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if s.bgLoader == nil {
		common.WriteError(w, http.StatusServiceUnavailable, "background loader not initialized")
		return
	}

	if err := s.bgLoader.Restart(); err != nil {
		common.WriteError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
		return
	}

	common.WriteSuccess(w, map[string]string{
		"message": "Index and books reloaded successfully. Background loading restarted.",
	})
}

// handleModeCompliance returns compliance check results for a single mode.
func (s *Server) handleModeCompliance(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if mode == "" {
		common.WriteError(w, http.StatusBadRequest, "mode parameter required")
		return
	}

	table, err := s.loader.GetMode(mode)
	if err != nil {
		common.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	checker := lut.NewComplianceChecker()
	result := checker.CheckMode(table)

	common.WriteSuccess(w, result)
}

// handleAllCompliance returns compliance check results for all modes.
func (s *Server) handleAllCompliance(w http.ResponseWriter, r *http.Request) {
	tables := make(map[string]*stakergs.LookupTable)

	for _, mode := range s.loader.ListModes() {
		table, err := s.loader.GetMode(mode)
		if err != nil {
			continue
		}
		tables[mode] = table
	}

	if len(tables) == 0 {
		common.WriteError(w, http.StatusNotFound, "no modes available")
		return
	}

	checker := lut.NewComplianceChecker()
	result := checker.CheckAllModes(tables)

	common.WriteSuccess(w, result)
}
