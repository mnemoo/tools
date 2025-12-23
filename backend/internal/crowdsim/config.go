// Package crowdsim provides multi-player simulation for evaluating weight distributions.
package crowdsim

import (
	"fmt"
	"runtime"
)

// SimConfig holds simulation parameters.
type SimConfig struct {
	PlayerCount     int     `json:"player_count"`      // Number of simulated players (1000-10000)
	SpinsPerSession int     `json:"spins_per_session"` // Spins per player session (1-1000)
	InitialBalance  float64 `json:"initial_balance"`   // Starting balance
	BetAmount       float64 `json:"bet_amount"`        // Bet per spin
	BigWinThreshold float64 `json:"big_win_threshold"` // Multiplier threshold for "big win" (e.g., 10.0)
	DangerThreshold float64 `json:"danger_threshold"`  // Balance fraction for "danger" events (e.g., 0.1)
	UseCryptoRNG    bool    `json:"use_crypto_rng"`    // Use crypto/rand for secure randomness
	StreamingMode   bool    `json:"streaming_mode"`    // Memory-efficient mode (no full history)
	ParallelWorkers int     `json:"parallel_workers"`  // Number of goroutines for simulation
}

// DefaultConfig returns a reasonable default configuration.
func DefaultConfig() SimConfig {
	return SimConfig{
		PlayerCount:     1000,
		SpinsPerSession: 200,
		InitialBalance:  100.0,
		BetAmount:       1.0,
		BigWinThreshold: 10.0,
		DangerThreshold: 0.1,
		UseCryptoRNG:    false,
		StreamingMode:   false,
		ParallelWorkers: runtime.NumCPU(),
	}
}

// Preset configurations for common use cases.
var (
	// PresetQuick is for rapid testing with fewer players.
	PresetQuick = SimConfig{
		PlayerCount:     500,
		SpinsPerSession: 100,
		InitialBalance:  100.0,
		BetAmount:       1.0,
		BigWinThreshold: 10.0,
		DangerThreshold: 0.1,
		UseCryptoRNG:    false,
		StreamingMode:   false,
		ParallelWorkers: runtime.NumCPU(),
	}

	// PresetStandard is the balanced default.
	PresetStandard = SimConfig{
		PlayerCount:     2000,
		SpinsPerSession: 200,
		InitialBalance:  100.0,
		BetAmount:       1.0,
		BigWinThreshold: 10.0,
		DangerThreshold: 0.1,
		UseCryptoRNG:    false,
		StreamingMode:   false,
		ParallelWorkers: runtime.NumCPU(),
	}

	// PresetThorough is for detailed analysis with more players.
	PresetThorough = SimConfig{
		PlayerCount:     5000,
		SpinsPerSession: 300,
		InitialBalance:  100.0,
		BetAmount:       1.0,
		BigWinThreshold: 10.0,
		DangerThreshold: 0.1,
		UseCryptoRNG:    true,
		StreamingMode:   false,
		ParallelWorkers: runtime.NumCPU(),
	}
)

// Validate checks if the configuration is valid and applies defaults.
func (c *SimConfig) Validate() error {
	if c.PlayerCount <= 0 {
		c.PlayerCount = 1000
	}
	if c.PlayerCount > 100000 {
		return fmt.Errorf("player_count exceeds maximum (100000): %d", c.PlayerCount)
	}

	if c.SpinsPerSession <= 0 {
		c.SpinsPerSession = 200
	}
	if c.SpinsPerSession > 10000 {
		return fmt.Errorf("spins_per_session exceeds maximum (10000): %d", c.SpinsPerSession)
	}

	if c.InitialBalance <= 0 {
		c.InitialBalance = 100.0
	}

	if c.BetAmount <= 0 {
		c.BetAmount = 1.0
	}

	if c.BetAmount > c.InitialBalance {
		return fmt.Errorf("bet_amount (%v) cannot exceed initial_balance (%v)", c.BetAmount, c.InitialBalance)
	}

	if c.BigWinThreshold <= 0 {
		c.BigWinThreshold = 10.0
	}

	if c.DangerThreshold <= 0 || c.DangerThreshold >= 1.0 {
		c.DangerThreshold = 0.1
	}

	if c.ParallelWorkers <= 0 {
		c.ParallelWorkers = runtime.NumCPU()
	}
	if c.ParallelWorkers > 64 {
		c.ParallelWorkers = 64
	}

	return nil
}

// RankingWeights holds weights for composite score calculation.
type RankingWeights struct {
	ProfitWeight     float64 `json:"profit_weight"`     // Weight for PoP
	SafetyWeight     float64 `json:"safety_weight"`     // Weight for inverse drawdown
	ExcitementWeight float64 `json:"excitement_weight"` // Weight for peak balance
	FrustrationPen   float64 `json:"frustration_pen"`   // Penalty for lose streaks
}

// DefaultRankingWeights returns balanced ranking weights.
func DefaultRankingWeights() RankingWeights {
	return RankingWeights{
		ProfitWeight:     1.0,
		SafetyWeight:     0.8,
		ExcitementWeight: 0.5,
		FrustrationPen:   0.3,
	}
}

// PresetInfo describes a preset configuration.
type PresetInfo struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Config      SimConfig `json:"config"`
}

// GetPresets returns all available presets.
func GetPresets() []PresetInfo {
	return []PresetInfo{
		{
			Name:        "quick",
			Description: "Fast simulation with 500 players, 100 spins",
			Config:      PresetQuick,
		},
		{
			Name:        "standard",
			Description: "Balanced simulation with 2000 players, 200 spins",
			Config:      PresetStandard,
		},
		{
			Name:        "thorough",
			Description: "Detailed analysis with 5000 players, 300 spins, crypto RNG",
			Config:      PresetThorough,
		},
	}
}
