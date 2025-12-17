package stakergs

// GameConfig represents the complete configuration for a game in Stake Engine.
type GameConfig struct {
	GameID      string         `json:"game_id"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	BetModes    *BetModeConfig `json:"bet_modes"`
	LookupTable *LookupTable   `json:"lookup_table"`
	SubGames    []SubGame      `json:"sub_games,omitempty"`
}

// SubGame represents a sub-game or bonus mode within a game.
type SubGame struct {
	ID          string       `json:"id"`   // e.g., "freegame", "bonus"
	Name        string       `json:"name"` // Display name
	TargetRTP   uint         `json:"target_rtp"`
	LookupTable *LookupTable `json:"lookup_table"`
}

// SpinResult represents the result of a single spin.
type SpinResult struct {
	OutcomeIndex int  `json:"outcome_index"`
	SimID        int  `json:"sim_id"`
	Payout       uint `json:"payout"`       // Multiplier * 100
	WinAmount    uint `json:"win_amount"`   // Actual win in currency units
	BetAmount    uint `json:"bet_amount"`   // Bet in currency units
	IsWin        bool `json:"is_win"`
}

// NewSpinResult creates a SpinResult from an outcome and bet.
func NewSpinResult(outcome *Outcome, betAmount uint) SpinResult {
	winAmount := (betAmount * outcome.Payout) / 100
	return SpinResult{
		SimID:     outcome.SimID,
		Payout:    outcome.Payout,
		WinAmount: winAmount,
		BetAmount: betAmount,
		IsWin:     outcome.Payout > 0,
	}
}

// PayoutFormat represents how payouts are stored.
// All payouts in stakergs are stored as multiplier * 100.
// Examples:
//   - 0    = 0x (loss)
//   - 50   = 0.5x
//   - 100  = 1x (break even)
//   - 150  = 1.5x
//   - 1000 = 10x
const PayoutMultiplier = 100

// RTPFormat represents how RTP values are stored.
// RTP is stored as percentage * 100.
// Examples:
//   - 9700 = 97.00%
//   - 9650 = 96.50%
const RTPMultiplier = 10000

// ToRTPPercent converts stored RTP to percentage (e.g., 9700 -> 97.00).
func ToRTPPercent(rtp uint) float64 {
	return float64(rtp) / 100.0
}

// FromRTPPercent converts percentage to stored RTP (e.g., 97.00 -> 9700).
func FromRTPPercent(percent float64) uint {
	return uint(percent * 100)
}

// ToPayoutMultiplier converts stored payout to multiplier (e.g., 150 -> 1.5).
func ToPayoutMultiplier(payout uint) float64 {
	return float64(payout) / float64(PayoutMultiplier)
}

// FromPayoutMultiplier converts multiplier to stored payout (e.g., 1.5 -> 150).
func FromPayoutMultiplier(multiplier float64) uint {
	return uint(multiplier * float64(PayoutMultiplier))
}
