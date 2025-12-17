package lgs

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultBalance is the initial balance ($1,000,000 in API units where 1000000 = $1)
const DefaultBalance = 1000000000000

// MaxHistorySize is the maximum number of rounds to keep in history
const MaxHistorySize = 100

// SessionData stores session information
type SessionData struct {
	SessionID    string
	Balance      int64
	Currency     string
	Language     string
	LastRound    *RoundInfo
	History      []RoundInfo
	BetIDCounter int64
	CreatedAt    time.Time
	LastActivity time.Time
	TotalBets    int64
	TotalWins    int64
	TotalWagered int64
	TotalWon     int64
	// ForcedSimID maps mode -> simID for forcing specific outcomes
	ForcedSimID map[string]int
	// RTPBias is an exponent that biases sampling toward higher payouts.
	// 0.0 = normal RTP, positive values boost high payouts (e.g., 0.5 = moderate boost, 1.0 = strong boost)
	// The weight for each outcome is multiplied by payout^RTPBias
	RTPBias float64
}

// NextBetID returns the simID as the bet ID
func (s *SessionData) NextBetID(simID int) int {
	s.BetIDCounter++
	return simID
}

// AddRound adds a round to history
func (s *SessionData) AddRound(round RoundInfo) {
	s.History = append(s.History, round)
	if len(s.History) > MaxHistorySize {
		s.History = s.History[len(s.History)-MaxHistorySize:]
	}
	s.LastRound = &round
	s.TotalBets++
	s.TotalWagered += round.Amount
	s.TotalWon += round.Payout
	if round.Payout > 0 {
		s.TotalWins++
	}
}

// GetStats returns session statistics
func (s *SessionData) GetStats() map[string]interface{} {
	hitRate := 0.0
	if s.TotalBets > 0 {
		hitRate = float64(s.TotalWins) / float64(s.TotalBets)
	}
	rtp := 0.0
	if s.TotalWagered > 0 {
		rtp = float64(s.TotalWon) / float64(s.TotalWagered)
	}
	return map[string]interface{}{
		"totalBets":    s.TotalBets,
		"totalWins":    s.TotalWins,
		"totalWagered": s.TotalWagered,
		"totalWon":     s.TotalWon,
		"hitRate":      hitRate,
		"rtp":          rtp,
	}
}

// ClearHistory clears round history
func (s *SessionData) ClearHistory() {
	s.History = make([]RoundInfo, 0)
	s.LastRound = nil
}

// ClearStats resets session statistics
func (s *SessionData) ClearStats() {
	s.TotalBets = 0
	s.TotalWins = 0
	s.TotalWagered = 0
	s.TotalWon = 0
}

// SetForcedSimID sets a specific simID to be used for the next play in a mode
func (s *SessionData) SetForcedSimID(mode string, simID int) {
	if s.ForcedSimID == nil {
		s.ForcedSimID = make(map[string]int)
	}
	// Store with lowercase key for case-insensitive matching
	s.ForcedSimID[strings.ToLower(mode)] = simID
}

// ConsumeForcedSimID returns and clears the forced simID for a mode
// Returns simID and true if set, 0 and false otherwise
func (s *SessionData) ConsumeForcedSimID(mode string) (int, bool) {
	if s.ForcedSimID == nil {
		return 0, false
	}
	modeLower := strings.ToLower(mode)
	simID, ok := s.ForcedSimID[modeLower]
	if ok {
		delete(s.ForcedSimID, modeLower)
	}
	return simID, ok
}

// GetForcedSimID returns the forced simID for a mode without consuming it
func (s *SessionData) GetForcedSimID(mode string) (int, bool) {
	if s.ForcedSimID == nil {
		return 0, false
	}
	simID, ok := s.ForcedSimID[strings.ToLower(mode)]
	return simID, ok
}

// ClearForcedSimID clears the forced simID for a mode
func (s *SessionData) ClearForcedSimID(mode string) {
	if s.ForcedSimID != nil {
		delete(s.ForcedSimID, strings.ToLower(mode))
	}
}

// GetAllForcedSimIDs returns all forced simIDs
func (s *SessionData) GetAllForcedSimIDs() map[string]int {
	if s.ForcedSimID == nil {
		return make(map[string]int)
	}
	result := make(map[string]int, len(s.ForcedSimID))
	for k, v := range s.ForcedSimID {
		result[k] = v
	}
	return result
}

// SessionManager manages player sessions
type SessionManager struct {
	sessions map[string]*SessionData
	mu       sync.RWMutex
	counter  atomic.Int64
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*SessionData),
	}
}

// GetOrCreate gets existing session or creates a new one
func (sm *SessionManager) GetOrCreate(sessionID string) *SessionData {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[sessionID]; ok {
		session.LastActivity = time.Now()
		return session
	}

	session := &SessionData{
		SessionID:    sessionID,
		Balance:      DefaultBalance,
		Currency:     "USD",
		Language:     "en",
		History:      make([]RoundInfo, 0),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	sm.sessions[sessionID] = session
	sm.counter.Add(1)

	return session
}

// Get returns session by ID or nil if not found
func (sm *SessionManager) Get(sessionID string) *SessionData {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.sessions[sessionID]
}

// Update updates a session
func (sm *SessionManager) Update(session *SessionData) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session.LastActivity = time.Now()
	sm.sessions[session.SessionID] = session
}

// Delete removes a session
func (sm *SessionManager) Delete(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, sessionID)
}

// Count returns the number of active sessions
func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return len(sm.sessions)
}

// GetAll returns all active sessions
func (sm *SessionManager) GetAll() []*SessionData {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*SessionData, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// TotalCreated returns total number of sessions created
func (sm *SessionManager) TotalCreated() int64 {
	return sm.counter.Load()
}

// ResetBalance resets session balance to default
func (sm *SessionManager) ResetBalance(sessionID string) *SessionData {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[sessionID]; ok {
		session.Balance = DefaultBalance
		session.LastActivity = time.Now()
		return session
	}

	return nil
}

// CleanupInactive removes sessions inactive for more than the given duration
func (sm *SessionManager) CleanupInactive(maxAge time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	for id, session := range sm.sessions {
		if session.LastActivity.Before(cutoff) {
			delete(sm.sessions, id)
			cleaned++
		}
	}

	return cleaned
}

// StartCleanupRoutine starts a background cleanup routine
func (sm *SessionManager) StartCleanupRoutine(interval, maxAge time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.CleanupInactive(maxAge)
		case <-stop:
			return
		}
	}
}
