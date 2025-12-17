// Package lgs provides Local Game Server functionality for LUT Explorer.
// This is a simplified LGS for testing slot games using pregenerated LUT data.
// Implements RGS-compatible endpoints: /wallet/* and /bet/*
package lgs

import (
	"encoding/json"

	"lutexplorer/internal/common"
)

// APIMultiplier (100 = 1x in payouts, amounts in cents)
const APIMultiplier = 100

// AuthRequest for /wallet/authenticate
type AuthRequest struct {
	SessionID string `json:"sessionID"`
	Language  string `json:"language"`
}

// AuthResponse for /wallet/authenticate
type AuthResponse struct {
	Balance BalanceInfo `json:"balance"`
	Round   interface{} `json:"round"`
	Config  ConfigInfo  `json:"config"`
	Meta    interface{} `json:"meta"`
}

// BalanceInfo represents balance information
type BalanceInfo struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// JurisdictionInfo represents jurisdiction settings
type JurisdictionInfo struct {
	SocialCasino         bool `json:"socialCasino"`
	DisabledFullscreen   bool `json:"disabledFullscreen"`
	DisabledTurbo        bool `json:"disabledTurbo"`
	DisabledSuperTurbo   bool `json:"disabledSuperTurbo"`
	DisabledAutoplay     bool `json:"disabledAutoplay"`
	DisabledSlamstop     bool `json:"disabledSlamstop"`
	DisabledSpacebar     bool `json:"disabledSpacebar"`
	DisabledBuyFeature   bool `json:"disabledBuyFeature"`
	DisplayNetPosition   bool `json:"displayNetPosition"`
	DisplayRTP           bool `json:"displayRTP"`
	DisplaySessionTimer  bool `json:"displaySessionTimer"`
	MinimumRoundDuration int  `json:"minimumRoundDuration"`
}

// ConfigInfo represents game configuration
type ConfigInfo struct {
	GameID          string                 `json:"gameID"`
	MinBet          int64                  `json:"minBet"`
	MaxBet          int64                  `json:"maxBet"`
	StepBet         int64                  `json:"stepBet"`
	DefaultBetLevel int64                  `json:"defaultBetLevel"`
	BetLevels       []int64                `json:"betLevels"`
	BetModes        map[string]interface{} `json:"betModes"`
	Jurisdiction    JurisdictionInfo       `json:"jurisdiction"`
}

// DefaultConfigInfo returns default game configuration.
func DefaultConfigInfo() ConfigInfo {
	return ConfigInfo{
		GameID:          "",
		MinBet:          common.MinBet,
		MaxBet:          common.MaxBet,
		StepBet:         common.StepBet,
		DefaultBetLevel: common.DefaultBetLevel,
		BetLevels:       common.DefaultBetLevels(),
		BetModes:        map[string]interface{}{},
		Jurisdiction:    JurisdictionInfo{},
	}
}

// PlayRequest for /wallet/play
type PlayRequest struct {
	Mode      string `json:"mode"`
	Currency  string `json:"currency"`
	SessionID string `json:"sessionID"`
	Amount    int64  `json:"amount"`
}

// PlayResponse for /wallet/play
type PlayResponse struct {
	Balance BalanceInfo `json:"balance"`
	Round   RoundInfo   `json:"round"`
}

// RoundInfo represents a game round
type RoundInfo struct {
	BetID            int             `json:"betID"`
	Amount           int64           `json:"amount"`
	Payout           int64           `json:"payout"`
	PayoutMultiplier float64         `json:"payoutMultiplier"`
	Active           bool            `json:"active"`
	State            json.RawMessage `json:"state"`
	Mode             string          `json:"mode"`
	Event            interface{}     `json:"event"`
}

// EndRoundRequest for /wallet/end-round
type EndRoundRequest struct {
	SessionID string `json:"sessionID"`
}

// EndRoundResponse for /wallet/end-round
type EndRoundResponse struct {
	Balance BalanceInfo `json:"balance"`
	Round   interface{} `json:"round"`
	Config  ConfigInfo  `json:"config"`
	Meta    interface{} `json:"meta"`
}

// EventRequest for /bet/event (end event)
type EventRequest struct {
	SessionID string `json:"sessionID"`
	Event     string `json:"event"` // Event index as string
}

// EventResponse for /bet/event - simple format
type EventResponse struct {
	Event string `json:"event"`
}

// ReplayResponse for /bet/replay/{game}/{version}/{mode}/{event}
type ReplayResponse struct {
	PayoutMultiplier float64         `json:"payoutMultiplier"`
	CostMultiplier   float64         `json:"costMultiplier"`
	State            json.RawMessage `json:"state"`
}

// HealthResponse for health check
type HealthResponse struct {
	Status       string         `json:"status"`
	Game         string         `json:"game"`
	ModesLoaded  []string       `json:"modesLoaded"`
	EventsLoaded map[string]int `json:"eventsLoaded"`
}

// ErrorResponse for errors
type ErrorResponse struct {
	Error   string `json:"error"`
	Success bool   `json:"success"`
}

// HistoryRequest for history
type HistoryRequest struct {
	SessionID string `json:"sessionID"`
	Limit     int    `json:"limit"`
}

// HistoryResponse for history
type HistoryResponse struct {
	Rounds  []RoundInfo `json:"rounds"`
	Balance BalanceInfo `json:"balance"`
}

// BatchPlayRequest for /lgs/batchplay - play multiple rounds at once
type BatchPlayRequest struct {
	SessionID string `json:"sessionID"`
	Mode      string `json:"mode"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Spins     int    `json:"spins"` // Number of spins to play
}

// BatchPlayRound contains result of a single round in batch
type BatchPlayRound struct {
	SpinNum          int     `json:"spinNum"`
	SimID            int     `json:"simID"`
	Payout           int64   `json:"payout"`
	PayoutMultiplier float64 `json:"payoutMultiplier"`
}

// BatchPlayResponse for /lgs/batchplay
type BatchPlayResponse struct {
	SessionID    string           `json:"sessionID"`
	Mode         string           `json:"mode"`
	Spins        int              `json:"spins"`
	TotalWagered int64            `json:"totalWagered"`
	TotalWon     int64            `json:"totalWon"`
	HitCount     int              `json:"hitCount"`
	HitRate      float64          `json:"hitRate"`
	RTP          float64          `json:"rtp"`
	MaxWin       float64          `json:"maxWin"`
	BigWins      int              `json:"bigWins"`  // >= 10x
	MegaWins     int              `json:"megaWins"` // >= 50x
	Balance      BalanceInfo      `json:"balance"`
	Rounds       []BatchPlayRound `json:"rounds,omitempty"` // Only if spins <= 1000
	DurationMs   int64            `json:"durationMs"`
}

// SessionSummary contains summary info for a single session
type SessionSummary struct {
	SessionID      string         `json:"sessionID"`
	Balance        int64          `json:"balance"`
	Currency       string         `json:"currency"`
	TotalBets      int64          `json:"totalBets"`
	TotalWins      int64          `json:"totalWins"`
	TotalWagered   int64          `json:"totalWagered"`
	TotalWon       int64          `json:"totalWon"`
	RTP            float64        `json:"rtp"`
	HitRate        float64        `json:"hitRate"`
	Profit         int64          `json:"profit"` // totalWagered - totalWon (house profit)
	HistorySize    int            `json:"historySize"`
	CreatedAt      string         `json:"createdAt"`
	LastActivity   string         `json:"lastActivity"`
	ForcedOutcomes map[string]int `json:"forcedOutcomes"`
	RTPBias        float64        `json:"rtpBias"`
}

// SessionsResponse for GET /lgs/sessions
type SessionsResponse struct {
	Sessions       []SessionSummary `json:"sessions"`
	TotalSessions  int              `json:"totalSessions"`
	TotalCreated   int64            `json:"totalCreated"`
	AggregateStats AggregateStats   `json:"aggregate"`
}

// AggregateStats contains aggregate statistics across all sessions
type AggregateStats struct {
	TotalBets      int64   `json:"totalBets"`
	TotalWins      int64   `json:"totalWins"`
	TotalWagered   int64   `json:"totalWagered"`
	TotalWon       int64   `json:"totalWon"`
	OverallRTP     float64 `json:"overallRTP"`
	OverallHitRate float64 `json:"overallHitRate"`
	TotalProfit    int64   `json:"totalProfit"`
}
