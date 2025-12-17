package stakergs

// BetMode represents a betting mode configuration.
type BetMode struct {
	Name      string `json:"name"`       // Display name (e.g., "Normal", "Turbo", "High Roller")
	ID        string `json:"id"`         // Unique identifier (e.g., "normal", "turbo", "high_roller")
	Cost      uint   `json:"cost"`       // Cost per spin in smallest currency unit (cents)
	MinBet    uint   `json:"min_bet"`    // Minimum bet amount
	MaxBet    uint   `json:"max_bet"`    // Maximum bet amount
	TargetRTP uint   `json:"target_rtp"` // Target RTP * 10000 (e.g., 9700 = 97.00%)
}

// BetModeConfig holds the configuration for all bet modes in a game.
type BetModeConfig struct {
	GameID   string    `json:"game_id"`
	Modes    []BetMode `json:"modes"`
	Default  string    `json:"default"` // Default mode ID
	Currency string    `json:"currency"`
}

// GetMode returns the BetMode by ID, or nil if not found.
func (c *BetModeConfig) GetMode(id string) *BetMode {
	for i := range c.Modes {
		if c.Modes[i].ID == id {
			return &c.Modes[i]
		}
	}
	return nil
}

// GetDefaultMode returns the default BetMode.
func (c *BetModeConfig) GetDefaultMode() *BetMode {
	return c.GetMode(c.Default)
}

// ValidateBet checks if a bet amount is valid for the given mode.
func (m *BetMode) ValidateBet(amount uint) bool {
	return amount >= m.MinBet && amount <= m.MaxBet
}

// CalculatePayout returns the payout for a given bet and payout multiplier.
// payoutMultiplier is in format multiplier * 100 (e.g., 150 = 1.5x).
func (m *BetMode) CalculatePayout(bet uint, payoutMultiplier uint) uint {
	return (bet * payoutMultiplier) / 100
}

// BetLevels represents predefined bet levels for quick selection.
type BetLevels struct {
	Levels []uint `json:"levels"` // Available bet amounts
}

// DefaultBetLevels returns common bet level presets.
func DefaultBetLevels() BetLevels {
	return BetLevels{
		Levels: []uint{
			10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000,
		},
	}
}
