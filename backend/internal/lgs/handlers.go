package lgs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"lutexplorer/internal/common"
	"lutexplorer/internal/lut"
	"lutexplorer/internal/ws"
	"stakergs"
)

// batchPlayStats holds statistics for batch play operations.
type batchPlayStats struct {
	totalWagered int64
	totalWon     int64
	hitCount     int
	bigWins      int
	megaWins     int
	maxWin       float64
}

// processBatchSpins executes multiple spins and returns statistics.
func processBatchSpins(
	session *SessionData,
	sampleOutcome func() stakergs.Outcome,
	spins int,
	betPerSpin int64,
	baseAmount int64,
	keepRounds bool,
) (batchPlayStats, []BatchPlayRound) {
	var stats batchPlayStats
	var rounds []BatchPlayRound
	if keepRounds {
		rounds = make([]BatchPlayRound, 0, spins)
	}

	for i := 0; i < spins; i++ {
		// Deduct bet
		session.Balance -= betPerSpin
		stats.totalWagered += betPerSpin

		// Sample outcome
		outcome := sampleOutcome()
		payoutMultiplier := float64(outcome.Payout) / 100.0
		payout := int64(float64(baseAmount) * payoutMultiplier)

		// Add payout
		session.Balance += payout
		stats.totalWon += payout

		// Track stats
		if payout > 0 {
			stats.hitCount++
		}
		if payoutMultiplier >= common.BigWinMultiplier {
			stats.bigWins++
		}
		if payoutMultiplier >= common.MegaWinMultiplier {
			stats.megaWins++
		}
		if payoutMultiplier > stats.maxWin {
			stats.maxWin = payoutMultiplier
		}

		// Keep round data if not too many spins
		if keepRounds {
			rounds = append(rounds, BatchPlayRound{
				SpinNum:          i + 1,
				SimID:            outcome.SimID,
				Payout:           payout,
				PayoutMultiplier: payoutMultiplier,
			})
		}

		// Update session stats
		session.TotalBets++
		session.TotalWagered += betPerSpin
		session.TotalWon += payout
		if payout > 0 {
			session.TotalWins++
		}
	}

	return stats, rounds
}

// extractEvents extracts only the "events" array from a book JSON.
// Returns the events array or empty array if not found.
func extractEvents(bookJSON json.RawMessage) json.RawMessage {
	if len(bookJSON) == 0 {
		return json.RawMessage(`[]`)
	}

	// Parse the book JSON to extract events field
	var book map[string]json.RawMessage
	if err := json.Unmarshal(bookJSON, &book); err != nil {
		// If it's not an object, return as-is (might already be events array)
		return bookJSON
	}

	// Look for "events" field
	if events, ok := book["events"]; ok {
		return events
	}

	// No events field found, return empty array
	return json.RawMessage(`[]`)
}

// Handlers holds all LGS HTTP handlers
type Handlers struct {
	loader   *lut.Loader
	sessions *SessionManager
	wsHub    *ws.Hub
}

// NewHandlers creates new LGS handlers
func NewHandlers(loader *lut.Loader, sessions *SessionManager, hub *ws.Hub) *Handlers {
	return &Handlers{
		loader:   loader,
		sessions: sessions,
		wsHub:    hub,
	}
}

// broadcastSessionsUpdate sends current sessions state to all WebSocket clients
func (h *Handlers) broadcastSessionsUpdate() {
	if h.wsHub == nil {
		return
	}

	allSessions := h.sessions.GetAll()
	summaries := make([]SessionSummary, 0, len(allSessions))

	var aggBets, aggWins, aggWagered, aggWon int64

	for _, s := range allSessions {
		rtp := 0.0
		if s.TotalWagered > 0 {
			rtp = float64(s.TotalWon) / float64(s.TotalWagered)
		}
		hitRate := 0.0
		if s.TotalBets > 0 {
			hitRate = float64(s.TotalWins) / float64(s.TotalBets)
		}
		profit := s.TotalWagered - s.TotalWon

		summaries = append(summaries, SessionSummary{
			SessionID:      s.SessionID,
			Balance:        s.Balance,
			Currency:       s.Currency,
			TotalBets:      s.TotalBets,
			TotalWins:      s.TotalWins,
			TotalWagered:   s.TotalWagered,
			TotalWon:       s.TotalWon,
			RTP:            rtp,
			HitRate:        hitRate,
			Profit:         profit,
			HistorySize:    len(s.History),
			CreatedAt:      s.CreatedAt.Format("2006-01-02 15:04:05"),
			LastActivity:   s.LastActivity.Format("2006-01-02 15:04:05"),
			ForcedOutcomes: s.GetAllForcedSimIDs(),
			RTPBias:        s.RTPBias,
		})

		aggBets += s.TotalBets
		aggWins += s.TotalWins
		aggWagered += s.TotalWagered
		aggWon += s.TotalWon
	}

	overallRTP := 0.0
	if aggWagered > 0 {
		overallRTP = float64(aggWon) / float64(aggWagered)
	}
	overallHitRate := 0.0
	if aggBets > 0 {
		overallHitRate = float64(aggWins) / float64(aggBets)
	}

	h.wsHub.Broadcast(ws.Message{
		Type: ws.MsgLGSSessionsUpdate,
		Payload: SessionsResponse{
			Sessions:      summaries,
			TotalSessions: len(allSessions),
			TotalCreated:  h.sessions.TotalCreated(),
			AggregateStats: AggregateStats{
				TotalBets:      aggBets,
				TotalWins:      aggWins,
				TotalWagered:   aggWagered,
				TotalWon:       aggWon,
				OverallRTP:     overallRTP,
				OverallHitRate: overallHitRate,
				TotalProfit:    aggWagered - aggWon,
			},
		},
	})
}

// sendJSON sends a JSON response
func (h *Handlers) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response
func (h *Handlers) sendError(w http.ResponseWriter, message string, status int) {
	h.sendJSON(w, ErrorResponse{Error: message, Success: false}, status)
}

// Health handles /lgs/health
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	eventsLoaded := make(map[string]int)
	// Note: events loading status could be tracked if needed

	h.sendJSON(w, HealthResponse{
		Status:       "ok",
		Game:         "lutexplorer",
		ModesLoaded:  h.loader.ListModes(),
		EventsLoaded: eventsLoaded,
	}, http.StatusOK)
}

// Authenticate handles /wallet/authenticate
func (h *Handlers) Authenticate(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.SessionID = "default-session"
		req.Language = "en"
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}
	if req.Language == "" {
		req.Language = "en"
	}

	session := h.sessions.GetOrCreate(req.SessionID)
	session.Language = req.Language

	fmt.Printf("[LGS] Authenticate: session=%s, balance=%d\n", req.SessionID, session.Balance)

	// Broadcast session update
	h.broadcastSessionsUpdate()

	h.sendJSON(w, AuthResponse{
		Balance: BalanceInfo{
			Amount:   session.Balance,
			Currency: session.Currency,
		},
		Round:  nil,
		Config: DefaultConfigInfo(),
		Meta:   nil,
	}, http.StatusOK)
}

// Play handles /lgs/play - spins the reels
func (h *Handlers) Play(w http.ResponseWriter, r *http.Request) {
	var req PlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}
	if req.Mode == "" {
		h.sendError(w, "mode is required", http.StatusBadRequest)
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.Amount == 0 {
		req.Amount = APIMultiplier
	}

	// Get session
	session := h.sessions.GetOrCreate(req.SessionID)
	session.Currency = req.Currency

	// Get LUT for mode
	table, err := h.loader.GetMode(req.Mode)
	if err != nil {
		h.sendError(w, fmt.Sprintf("mode not found: %s", req.Mode), http.StatusBadRequest)
		return
	}

	// Calculate total bet (amount * mode cost)
	modeCost := table.Cost
	if modeCost == 0 {
		modeCost = 1.0
	}
	totalBet := int64(float64(req.Amount) * modeCost)

	// Check balance
	if session.Balance < totalBet {
		h.sendError(w, "insufficient balance", http.StatusBadRequest)
		return
	}

	// Deduct bet
	session.Balance -= totalBet

	// Check for forced outcome first
	var outcome stakergs.Outcome
	var forced bool
	if forcedSimID, ok := session.ConsumeForcedSimID(req.Mode); ok {
		// Find the outcome with this simID
		for _, o := range table.Outcomes {
			if o.SimID == forcedSimID {
				outcome = o
				forced = true
				break
			}
		}
		if !forced {
			h.sendError(w, fmt.Sprintf("forced simID %d not found in mode %s", forcedSimID, req.Mode), http.StatusBadRequest)
			// Refund the bet
			session.Balance += totalBet
			return
		}
	} else {
		// Use weighted random selection, with bias if set
		if session.RTPBias != 0 {
			sampler := lut.NewBiasedWeightedSampler(table, session.RTPBias)
			outcome = sampler.SampleWithNewRNG()
		} else {
			sampler := lut.NewWeightedSampler(table)
			outcome = sampler.SampleWithNewRNG()
		}
	}

	// Calculate payout
	payoutMultiplier := float64(outcome.Payout) / 100.0
	payout := int64(float64(req.Amount) * payoutMultiplier)

	// Add payout to balance
	session.Balance += payout

	// Get event data (state) if available - extract only "events" array from book
	var stateData json.RawMessage
	eventsLoader := h.loader.EventsLoader()
	if bookJSON, err := eventsLoader.GetEvent(req.Mode, outcome.SimID, table.SimIDOffset); err == nil {
		stateData = extractEvents(bookJSON)
	} else {
		// Create minimal state if no event data
		stateData = json.RawMessage(`[]`)
	}

	// Create round info
	betID := session.NextBetID(outcome.SimID)
	roundInfo := RoundInfo{
		BetID:            betID,
		Amount:           totalBet,
		Payout:           payout,
		PayoutMultiplier: payoutMultiplier,
		Active:           true,
		State:            stateData,
		Mode:             req.Mode,
		Event:            nil,
	}

	// Add to history
	session.AddRound(roundInfo)
	h.sessions.Update(session)

	tag := ""
	if forced {
		tag = " [FORCED]"
	} else if session.RTPBias != 0 {
		tag = fmt.Sprintf(" [BIAS=%.2f]", session.RTPBias)
	}
	fmt.Printf("[LGS] Play: session=%s, mode=%s, bet=%d, simID=%d, payout=%d (%.2fx)%s\n",
		req.SessionID, req.Mode, totalBet, outcome.SimID, payout, payoutMultiplier, tag)

	// Broadcast session update
	h.broadcastSessionsUpdate()

	h.sendJSON(w, PlayResponse{
		Balance: BalanceInfo{
			Amount:   session.Balance,
			Currency: session.Currency,
		},
		Round: roundInfo,
	}, http.StatusOK)
}

// EndRound handles /wallet/end-round
func (h *Handlers) EndRound(w http.ResponseWriter, r *http.Request) {
	var req EndRoundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.SessionID = "default-session"
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}

	session := h.sessions.GetOrCreate(req.SessionID)

	// Mark round as inactive
	if session.LastRound != nil {
		session.LastRound.Active = false
	}

	fmt.Printf("[LGS] End Round: session=%s, balance=%d\n", req.SessionID, session.Balance)

	h.sendJSON(w, EndRoundResponse{
		Balance: BalanceInfo{
			Amount:   session.Balance,
			Currency: session.Currency,
		},
		Round:  nil,
		Config: DefaultConfigInfo(),
		Meta:   nil,
	}, http.StatusOK)
}

// History handles /lgs/history - returns round history
func (h *Handlers) History(w http.ResponseWriter, r *http.Request) {
	var req HistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.SessionID = "default-session"
		req.Limit = 50
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}
	if req.Limit <= 0 || req.Limit > MaxHistorySize {
		req.Limit = 50
	}

	session := h.sessions.GetOrCreate(req.SessionID)

	// Get last N rounds
	rounds := session.History
	if len(rounds) > req.Limit {
		rounds = rounds[len(rounds)-req.Limit:]
	}

	h.sendJSON(w, HistoryResponse{
		Rounds: rounds,
		Balance: BalanceInfo{
			Amount:   session.Balance,
			Currency: session.Currency,
		},
	}, http.StatusOK)
}

// Stats handles /lgs/stats - returns session statistics
func (h *Handlers) Stats(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		sessionID = "default-session"
	}

	session := h.sessions.Get(sessionID)
	if session == nil {
		h.sendError(w, "session not found", http.StatusNotFound)
		return
	}

	stats := session.GetStats()
	stats["balance"] = session.Balance
	stats["currency"] = session.Currency

	h.sendJSON(w, stats, http.StatusOK)
}

// ResetBalance handles reset-balance
func (h *Handlers) ResetBalance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.SessionID = "default-session"
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}

	session := h.sessions.ResetBalance(req.SessionID)
	if session == nil {
		session = h.sessions.GetOrCreate(req.SessionID)
	}

	fmt.Printf("[LGS] Reset Balance: session=%s, balance=%d\n", req.SessionID, session.Balance)

	// Broadcast session update
	h.broadcastSessionsUpdate()

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"balance": BalanceInfo{
			Amount:   session.Balance,
			Currency: session.Currency,
		},
	}, http.StatusOK)
}

// SetBalance handles set-balance - sets a specific balance for a session
func (h *Handlers) SetBalance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionID"`
		Balance   int64  `json:"balance"`
		Currency  string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}

	if req.Balance < 0 {
		h.sendError(w, "balance must be non-negative", http.StatusBadRequest)
		return
	}

	session := h.sessions.GetOrCreate(req.SessionID)
	session.Balance = req.Balance
	if req.Currency != "" {
		session.Currency = req.Currency
	}
	h.sessions.Update(session)

	fmt.Printf("[LGS] Set Balance: session=%s, balance=%d, currency=%s\n", req.SessionID, session.Balance, session.Currency)

	// Broadcast session update
	h.broadcastSessionsUpdate()

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"balance": BalanceInfo{
			Amount:   session.Balance,
			Currency: session.Currency,
		},
	}, http.StatusOK)
}

// Event handles /bet/event - ends an event (for multi-stage games)
func (h *Handlers) Event(w http.ResponseWriter, r *http.Request) {
	var req EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}

	// For simple slot games, event just acknowledges the event was processed
	// In more complex games, this would advance game state
	fmt.Printf("[LGS] Event: session=%s, event=%s\n", req.SessionID, req.Event)

	// Return simple event response
	h.sendJSON(w, EventResponse{
		Event: req.Event,
	}, http.StatusOK)
}

// BatchPlay handles POST /lgs/batchplay - plays multiple rounds at once
func (h *Handlers) BatchPlay(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req BatchPlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}
	if req.Mode == "" {
		h.sendError(w, "mode is required", http.StatusBadRequest)
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.Amount == 0 {
		req.Amount = APIMultiplier
	}
	if req.Spins <= 0 {
		req.Spins = 100
	}
	if req.Spins > 100000 {
		req.Spins = 100000
	}

	// Get session
	session := h.sessions.GetOrCreate(req.SessionID)
	session.Currency = req.Currency

	// Get LUT for mode
	table, err := h.loader.GetMode(req.Mode)
	if err != nil {
		h.sendError(w, fmt.Sprintf("mode not found: %s", req.Mode), http.StatusBadRequest)
		return
	}

	// Calculate bet per spin
	modeCost := table.Cost
	if modeCost == 0 {
		modeCost = 1.0
	}
	betPerSpin := int64(float64(req.Amount) * modeCost)
	totalBetRequired := betPerSpin * int64(req.Spins)

	// Check balance
	if session.Balance < totalBetRequired {
		h.sendError(w, fmt.Sprintf("insufficient balance: need %d, have %d", totalBetRequired, session.Balance), http.StatusBadRequest)
		return
	}

	// Create weighted sampler - use biased sampler if RTP bias is set
	var sampleOutcome func() stakergs.Outcome
	if session.RTPBias != 0 {
		biasedSampler := lut.NewBiasedWeightedSampler(table, session.RTPBias)
		sampleOutcome = biasedSampler.SampleWithNewRNG
	} else {
		regularSampler := lut.NewWeightedSampler(table)
		sampleOutcome = regularSampler.SampleWithNewRNG
	}

	// Play all spins
	keepRounds := req.Spins <= 1000
	stats, rounds := processBatchSpins(session, sampleOutcome, req.Spins, betPerSpin, req.Amount, keepRounds)

	h.sessions.Update(session)

	// Calculate rates
	rtp := 0.0
	if stats.totalWagered > 0 {
		rtp = float64(stats.totalWon) / float64(stats.totalWagered)
	}
	hitRate := 0.0
	if req.Spins > 0 {
		hitRate = float64(stats.hitCount) / float64(req.Spins)
	}

	durationMs := time.Since(start).Milliseconds()

	biasTag := ""
	if session.RTPBias != 0 {
		biasTag = fmt.Sprintf(" [BIAS=%.2f]", session.RTPBias)
	}
	fmt.Printf("[LGS] BatchPlay: session=%s, mode=%s, spins=%d, rtp=%.4f, duration=%dms%s\n",
		req.SessionID, req.Mode, req.Spins, rtp, durationMs, biasTag)

	// Broadcast session update
	h.broadcastSessionsUpdate()

	h.sendJSON(w, BatchPlayResponse{
		SessionID:    req.SessionID,
		Mode:         req.Mode,
		Spins:        req.Spins,
		TotalWagered: stats.totalWagered,
		TotalWon:     stats.totalWon,
		HitCount:     stats.hitCount,
		HitRate:      hitRate,
		RTP:          rtp,
		MaxWin:       stats.maxWin,
		BigWins:      stats.bigWins,
		MegaWins:     stats.megaWins,
		Balance: BalanceInfo{
			Amount:   session.Balance,
			Currency: session.Currency,
		},
		Rounds:     rounds,
		DurationMs: durationMs,
	}, http.StatusOK)
}

// Sessions handles GET /lgs/sessions - returns all active sessions with RTP
func (h *Handlers) Sessions(w http.ResponseWriter, r *http.Request) {
	allSessions := h.sessions.GetAll()

	summaries := make([]SessionSummary, 0, len(allSessions))

	// Aggregate stats
	var aggBets, aggWins, aggWagered, aggWon int64

	for _, s := range allSessions {
		rtp := 0.0
		if s.TotalWagered > 0 {
			rtp = float64(s.TotalWon) / float64(s.TotalWagered)
		}
		hitRate := 0.0
		if s.TotalBets > 0 {
			hitRate = float64(s.TotalWins) / float64(s.TotalBets)
		}
		profit := s.TotalWagered - s.TotalWon

		summaries = append(summaries, SessionSummary{
			SessionID:      s.SessionID,
			Balance:        s.Balance,
			Currency:       s.Currency,
			TotalBets:      s.TotalBets,
			TotalWins:      s.TotalWins,
			TotalWagered:   s.TotalWagered,
			TotalWon:       s.TotalWon,
			RTP:            rtp,
			HitRate:        hitRate,
			Profit:         profit,
			HistorySize:    len(s.History),
			CreatedAt:      s.CreatedAt.Format("2006-01-02 15:04:05"),
			LastActivity:   s.LastActivity.Format("2006-01-02 15:04:05"),
			ForcedOutcomes: s.GetAllForcedSimIDs(),
			RTPBias:        s.RTPBias,
		})

		// Accumulate for aggregate
		aggBets += s.TotalBets
		aggWins += s.TotalWins
		aggWagered += s.TotalWagered
		aggWon += s.TotalWon
	}

	// Calculate aggregate stats
	overallRTP := 0.0
	if aggWagered > 0 {
		overallRTP = float64(aggWon) / float64(aggWagered)
	}
	overallHitRate := 0.0
	if aggBets > 0 {
		overallHitRate = float64(aggWins) / float64(aggBets)
	}

	fmt.Printf("[LGS] Sessions: count=%d, totalBets=%d, overallRTP=%.4f\n",
		len(allSessions), aggBets, overallRTP)

	h.sendJSON(w, SessionsResponse{
		Sessions:      summaries,
		TotalSessions: len(allSessions),
		TotalCreated:  h.sessions.TotalCreated(),
		AggregateStats: AggregateStats{
			TotalBets:      aggBets,
			TotalWins:      aggWins,
			TotalWagered:   aggWagered,
			TotalWon:       aggWon,
			OverallRTP:     overallRTP,
			OverallHitRate: overallHitRate,
			TotalProfit:    aggWagered - aggWon,
		},
	}, http.StatusOK)
}

// ClearHistory handles DELETE /lgs/history - clears round history
func (h *Handlers) ClearHistory(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		sessionID = "default-session"
	}

	session := h.sessions.Get(sessionID)
	if session == nil {
		h.sendError(w, "session not found", http.StatusNotFound)
		return
	}

	session.ClearHistory()
	h.sessions.Update(session)

	fmt.Printf("[LGS] Clear History: session=%s\n", sessionID)

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"message": "history cleared",
	}, http.StatusOK)
}

// ClearStats handles DELETE /lgs/stats - clears session statistics
func (h *Handlers) ClearStats(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		sessionID = "default-session"
	}

	session := h.sessions.Get(sessionID)
	if session == nil {
		h.sendError(w, "session not found", http.StatusNotFound)
		return
	}

	session.ClearStats()
	h.sessions.Update(session)

	fmt.Printf("[LGS] Clear Stats: session=%s\n", sessionID)

	// Broadcast session update
	h.broadcastSessionsUpdate()

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"message": "stats cleared",
	}, http.StatusOK)
}

// ForceOutcome handles POST /lgs/force-outcome - sets the next spin outcome for a session/mode
func (h *Handlers) ForceOutcome(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionID"`
		Mode      string `json:"mode"`
		SimID     int    `json:"simID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}
	if req.Mode == "" {
		h.sendError(w, "mode is required", http.StatusBadRequest)
		return
	}

	// Verify the simID exists in the mode's LUT
	table, err := h.loader.GetMode(req.Mode)
	if err != nil {
		h.sendError(w, fmt.Sprintf("mode not found: %s", req.Mode), http.StatusBadRequest)
		return
	}

	var found bool
	var payout float64
	for _, o := range table.Outcomes {
		if o.SimID == req.SimID {
			found = true
			payout = float64(o.Payout) / 100.0
			break
		}
	}
	if !found {
		h.sendError(w, fmt.Sprintf("simID %d not found in mode %s", req.SimID, req.Mode), http.StatusBadRequest)
		return
	}

	session := h.sessions.GetOrCreate(req.SessionID)
	session.SetForcedSimID(req.Mode, req.SimID)
	h.sessions.Update(session)

	fmt.Printf("[LGS] Force Outcome: session=%s, mode=%s, simID=%d, payout=%.2fx\n",
		req.SessionID, req.Mode, req.SimID, payout)

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("next spin in %s will use simID %d (%.2fx)", req.Mode, req.SimID, payout),
		"mode":    req.Mode,
		"simID":   req.SimID,
		"payout":  payout,
	}, http.StatusOK)
}

// ClearForcedOutcome handles DELETE /lgs/force-outcome - clears forced outcome for a session/mode
func (h *Handlers) ClearForcedOutcome(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	mode := r.URL.Query().Get("mode")

	if sessionID == "" {
		sessionID = "default-session"
	}

	session := h.sessions.Get(sessionID)
	if session == nil {
		h.sendError(w, "session not found", http.StatusNotFound)
		return
	}

	if mode != "" {
		session.ClearForcedSimID(mode)
	} else {
		// Clear all forced outcomes
		session.ForcedSimID = nil
	}
	h.sessions.Update(session)

	fmt.Printf("[LGS] Clear Forced Outcome: session=%s, mode=%s\n", sessionID, mode)

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"message": "forced outcome cleared",
	}, http.StatusOK)
}

// GetForcedOutcomes handles GET /lgs/force-outcome - returns all forced outcomes for a session
func (h *Handlers) GetForcedOutcomes(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		sessionID = "default-session"
	}

	session := h.sessions.Get(sessionID)
	if session == nil {
		h.sendJSON(w, map[string]interface{}{
			"sessionID":      sessionID,
			"forcedOutcomes": map[string]int{},
		}, http.StatusOK)
		return
	}

	h.sendJSON(w, map[string]interface{}{
		"sessionID":      sessionID,
		"forcedOutcomes": session.GetAllForcedSimIDs(),
	}, http.StatusOK)
}

// SetRTPBias handles POST /lgs/rtp-bias - sets the RTP bias for a session
func (h *Handlers) SetRTPBias(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string  `json:"sessionID"`
		Bias      float64 `json:"bias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		req.SessionID = "default-session"
	}

	// Clamp bias to reasonable range [-2, 2]
	if req.Bias < -2 {
		req.Bias = -2
	}
	if req.Bias > 2 {
		req.Bias = 2
	}

	session := h.sessions.GetOrCreate(req.SessionID)
	session.RTPBias = req.Bias
	h.sessions.Update(session)

	fmt.Printf("[LGS] Set RTP Bias: session=%s, bias=%.2f\n", req.SessionID, req.Bias)

	// Broadcast session update
	h.broadcastSessionsUpdate()

	h.sendJSON(w, map[string]interface{}{
		"success":   true,
		"sessionID": req.SessionID,
		"bias":      req.Bias,
		"message":   fmt.Sprintf("RTP bias set to %.2f", req.Bias),
	}, http.StatusOK)
}

// GetRTPBias handles GET /lgs/rtp-bias - returns the RTP bias for a session
func (h *Handlers) GetRTPBias(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		sessionID = "default-session"
	}

	session := h.sessions.Get(sessionID)
	if session == nil {
		h.sendJSON(w, map[string]interface{}{
			"sessionID": sessionID,
			"bias":      0.0,
		}, http.StatusOK)
		return
	}

	h.sendJSON(w, map[string]interface{}{
		"sessionID": sessionID,
		"bias":      session.RTPBias,
	}, http.StatusOK)
}

// Replay handles /bet/replay/{game}/{version}/{mode}/{event} - returns event data for replay
func (h *Handlers) Replay(w http.ResponseWriter, r *http.Request) {
	game := r.PathValue("game")
	version := r.PathValue("version")
	mode := r.PathValue("mode")
	eventStr := r.PathValue("event")

	if game == "" || version == "" || mode == "" || eventStr == "" {
		h.sendError(w, "missing path parameters", http.StatusBadRequest)
		return
	}

	// Parse event as simID
	var simID int
	if _, err := fmt.Sscanf(eventStr, "%d", &simID); err != nil {
		h.sendError(w, "invalid event ID", http.StatusBadRequest)
		return
	}

	// Get mode table for payout info
	table, err := h.loader.GetMode(mode)
	if err != nil {
		h.sendError(w, fmt.Sprintf("mode not found: %s", mode), http.StatusNotFound)
		return
	}

	// Find payout multiplier for this simID
	var payoutMultiplier float64
	for _, o := range table.Outcomes {
		if o.SimID == simID {
			payoutMultiplier = float64(o.Payout) / 100.0
			break
		}
	}

	// Get cost multiplier from mode
	costMultiplier := table.Cost
	if costMultiplier == 0 {
		costMultiplier = 1.0
	}

	// Check if events are loaded for mode
	eventsLoader := h.loader.EventsLoader()
	if !eventsLoader.IsLoaded(mode) {
		// Try to load events
		if err := h.loader.LoadEvents(mode); err != nil {
			h.sendError(w, fmt.Sprintf("events not available for mode: %s", mode), http.StatusNotFound)
			return
		}
	}

	// Get event data (state) - extract only "events" array from book
	// Use SimIDOffset for backwards compatibility with old (1-indexed) and new (0-indexed) formats
	bookJSON, err := eventsLoader.GetEvent(mode, simID, table.SimIDOffset)
	if err != nil {
		h.sendError(w, fmt.Sprintf("event not found: %v", err), http.StatusNotFound)
		return
	}
	stateData := extractEvents(bookJSON)

	fmt.Printf("[LGS] Replay: game=%s, version=%s, mode=%s, simID=%d, payout=%.2fx\n", game, version, mode, simID, payoutMultiplier)

	h.sendJSON(w, ReplayResponse{
		PayoutMultiplier: payoutMultiplier,
		CostMultiplier:   costMultiplier,
		State:            stateData,
	}, http.StatusOK)
}
