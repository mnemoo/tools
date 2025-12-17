// Package stakergs provides core types and utilities for the Stake Engine.
package stakergs

// GameIndex represents the game manifest (index.json) that defines all game modes.
// This is the de-facto standard format for game configuration.
type GameIndex struct {
	Modes []ModeConfig `json:"modes"`
}

// ModeConfig defines a single game mode within the index.
type ModeConfig struct {
	Name    string  `json:"name"`    // Mode name: "base", "bonus", "freegame", etc.
	Cost    float64 `json:"cost"`    // Cost per spin in base units
	Events  string  `json:"events"`  // Path to events file (e.g., "books_base.jsonl.zst")
	Weights string  `json:"weights"` // Path to LUT CSV file (e.g., "lookUpTable_base_0.csv")
}

// LookupTable represents the complete set of game outcomes for a specific mode.
// Used by Stake Engine to determine payouts based on weighted random selection.
type LookupTable struct {
	Outcomes []Outcome `json:"outcomes"`
	GameID   string    `json:"game_id"`
	Mode     string    `json:"mode"` // e.g., "base", "bonus", "freegame"
	Cost     float64   `json:"cost"` // Cost per spin
}

// Outcome represents a single simulation outcome with weight and payout.
// CSV format: sim_id,weight,payout (e.g., "1,1996456169598,0")
type Outcome struct {
	SimID  int    `json:"sim_id"` // Simulation/payline number (1-based in CSV)
	Weight uint64 `json:"weight"` // Selection weight (uint64 for large values)
	Payout uint   `json:"payout"` // Payout multiplier * 100 (e.g., 550 = 5.50x)
}

// TotalWeight returns the sum of all outcome weights.
func (lut *LookupTable) TotalWeight() uint64 {
	var total uint64
	for _, o := range lut.Outcomes {
		total += o.Weight
	}
	return total
}

// SelectOutcome returns the outcome for a given random value in range [0, TotalWeight).
func (lut *LookupTable) SelectOutcome(rnd uint64) *Outcome {
	var cumulative uint64
	for i := range lut.Outcomes {
		cumulative += lut.Outcomes[i].Weight
		if rnd < cumulative {
			return &lut.Outcomes[i]
		}
	}
	// Fallback to last outcome (should not happen with valid rnd)
	if len(lut.Outcomes) > 0 {
		return &lut.Outcomes[len(lut.Outcomes)-1]
	}
	return nil
}

// RTP calculates the theoretical Return To Player as a decimal (e.g., 0.97 = 97%).
// For modes with cost > 1, RTP is adjusted: RTP = rawRTP / cost
// Example: bonus with cost=350x and avg payout 340.55x -> RTP = 340.55/350 = 0.9730 (97.30%)
func (lut *LookupTable) RTP() float64 {
	totalWeight := lut.TotalWeight()
	if totalWeight == 0 {
		return 0
	}

	// Use float64 accumulation to handle large weights without overflow
	var weightedPayoutSum float64
	for _, o := range lut.Outcomes {
		weightedPayoutSum += float64(o.Weight) * float64(o.Payout)
	}

	// Payout is multiplier * 100, so divide by 100 to get raw RTP
	rawRTP := weightedPayoutSum / float64(totalWeight) / 100.0

	// Adjust for cost: RTP = rawRTP / cost
	cost := lut.Cost
	if cost <= 0 {
		cost = 1.0
	}

	return rawRTP / cost
}

// HitRate returns the probability of a winning outcome (payout > 0).
func (lut *LookupTable) HitRate() float64 {
	totalWeight := lut.TotalWeight()
	if totalWeight == 0 {
		return 0
	}

	var winWeight uint64
	for _, o := range lut.Outcomes {
		if o.Payout > 0 {
			winWeight += o.Weight
		}
	}

	return float64(winWeight) / float64(totalWeight)
}

// MaxPayout returns the maximum payout multiplier * 100.
func (lut *LookupTable) MaxPayout() uint {
	var max uint
	for _, o := range lut.Outcomes {
		if o.Payout > max {
			max = o.Payout
		}
	}
	return max
}
