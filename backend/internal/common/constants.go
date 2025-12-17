// Package common provides shared utilities and constants for lutexplorer.
package common

// Simulation limits
const (
	DefaultSpins  = 1000
	MaxSpins      = 100000
	DefaultTrials = 100
	MaxTrials     = 10000
)

// Bet configuration (in cents, where 100000 = 1000.00)
const (
	MinBet          = 100000
	MaxBet          = 10000000
	StepBet         = 100000
	DefaultBetLevel = 1000000
)

// Win thresholds for classification
const (
	BigWinMultiplier  = 10.0
	MegaWinMultiplier = 50.0
)

// Optimizer constants
const (
	BaseWeight uint64 = 1_000_000_000_000 // 1 trillion base for weight calculations
)

// DefaultBetLevels returns standard bet levels for RGS compatibility.
func DefaultBetLevels() []int64 {
	return []int64{
		100000, 200000, 400000, 600000, 800000,
		1000000, 1200000, 1400000, 1600000, 1800000,
		2000000, 3000000, 4000000, 5000000, 6000000,
		7000000, 8000000, 9000000, 10000000,
	}
}

// BalanceRange represents a range for balance histogram.
type BalanceRange struct {
	Start float64
	End   float64
}

// DefaultBalanceRanges returns standard ranges for balance distribution analysis.
func DefaultBalanceRanges() []BalanceRange {
	return []BalanceRange{
		{0, 25},
		{25, 50},
		{50, 75},
		{75, 100},
		{100, 125},
		{125, 150},
		{150, 200},
		{200, 300},
		{300, 500},
		{500, 1000},
	}
}
