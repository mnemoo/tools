package crowdsim

// Player represents a single simulated player session.
type Player struct {
	ID              int       // Player identifier
	InitialBalance  float64   // Starting balance
	CurrentBalance  float64   // Current balance
	BalanceHistory  []float64 // Balance after each spin (nil in streaming mode)
	PeakBalance     float64   // Maximum balance reached
	MinBalance      float64   // Minimum balance reached
	MaxDrawdown     float64   // Maximum drawdown (initial - min) / initial
	CurrentStreak   int       // Current streak: positive=winning, negative=losing
	MaxWinStreak    int       // Longest winning streak
	MaxLoseStreak   int       // Longest losing streak
	TotalWins       int       // Count of winning spins (payout > bet)
	TotalLosses     int       // Count of losing spins (payout <= bet)
	FirstBigWinSpin int       // Spin number of first big win (-1 if never)
	DangerEvents    int       // Count of spins where balance was below danger threshold
	TotalWagered    float64   // Total amount bet
	TotalWon        float64   // Total payouts received
}

// NewPlayer creates a new player with initial balance.
func NewPlayer(id int, initialBalance float64, trackHistory bool, historySize int) *Player {
	p := &Player{
		ID:              id,
		InitialBalance:  initialBalance,
		CurrentBalance:  initialBalance,
		PeakBalance:     initialBalance,
		MinBalance:      initialBalance,
		FirstBigWinSpin: -1,
	}

	if trackHistory && historySize > 0 {
		p.BalanceHistory = make([]float64, 0, historySize+1)
		p.BalanceHistory = append(p.BalanceHistory, initialBalance)
	}

	return p
}

// ProcessSpin updates player state after a spin.
// spinNum is 0-indexed spin number.
// payout is the multiplier (e.g., 0.0, 1.5, 10.0).
// betAmount is the bet size.
// bigWinThreshold is the multiplier threshold for "big win".
// dangerThreshold is the fraction of initial balance considered "danger zone".
func (p *Player) ProcessSpin(spinNum int, payout, betAmount, bigWinThreshold, dangerThreshold float64) {
	// Deduct bet
	p.CurrentBalance -= betAmount
	p.TotalWagered += betAmount

	// Add payout
	winAmount := payout * betAmount
	p.CurrentBalance += winAmount
	p.TotalWon += winAmount

	// Track history
	if p.BalanceHistory != nil {
		p.BalanceHistory = append(p.BalanceHistory, p.CurrentBalance)
	}

	// Update peak/min
	if p.CurrentBalance > p.PeakBalance {
		p.PeakBalance = p.CurrentBalance
	}
	if p.CurrentBalance < p.MinBalance {
		p.MinBalance = p.CurrentBalance
	}

	// Update max drawdown
	drawdown := (p.InitialBalance - p.MinBalance) / p.InitialBalance
	if drawdown > p.MaxDrawdown {
		p.MaxDrawdown = drawdown
	}

	// Check danger zone
	dangerLevel := p.InitialBalance * dangerThreshold
	if p.CurrentBalance < dangerLevel {
		p.DangerEvents++
	}

	// Check for big win (first time)
	if p.FirstBigWinSpin == -1 && payout >= bigWinThreshold {
		p.FirstBigWinSpin = spinNum
	}

	// Update win/lose counts and streaks
	isWin := payout > 1.0 // Win if payout multiplier > 1x (net profit)

	if isWin {
		p.TotalWins++
		if p.CurrentStreak > 0 {
			p.CurrentStreak++
		} else {
			// Was losing or starting, now winning
			p.CurrentStreak = 1
		}
		if p.CurrentStreak > p.MaxWinStreak {
			p.MaxWinStreak = p.CurrentStreak
		}
	} else {
		p.TotalLosses++
		if p.CurrentStreak < 0 {
			p.CurrentStreak--
		} else {
			// Was winning or starting, now losing
			p.CurrentStreak = -1
		}
		if -p.CurrentStreak > p.MaxLoseStreak {
			p.MaxLoseStreak = -p.CurrentStreak
		}
	}
}

// FinalProfit returns the net profit (final balance - initial balance).
func (p *Player) FinalProfit() float64 {
	return p.CurrentBalance - p.InitialBalance
}

// IsProfitable returns true if player ended with at least their starting balance.
func (p *Player) IsProfitable() bool {
	return p.CurrentBalance >= p.InitialBalance
}

// ActualRTP returns the actual RTP for this player session.
func (p *Player) ActualRTP() float64 {
	if p.TotalWagered == 0 {
		return 0
	}
	return p.TotalWon / p.TotalWagered
}

// BalanceAtSpin returns balance at a specific spin (0 = initial).
// Returns -1 if history not tracked or spin out of range.
func (p *Player) BalanceAtSpin(spin int) float64 {
	if p.BalanceHistory == nil || spin < 0 || spin >= len(p.BalanceHistory) {
		return -1
	}
	return p.BalanceHistory[spin]
}

// HitBigWin returns true if player ever hit a big win.
func (p *Player) HitBigWin() bool {
	return p.FirstBigWinSpin >= 0
}

// PlayerSummary is a condensed view of player results.
type PlayerSummary struct {
	ID            int     `json:"id"`
	FinalBalance  float64 `json:"final_balance"`
	PeakBalance   float64 `json:"peak_balance"`
	MinBalance    float64 `json:"min_balance"`
	MaxDrawdown   float64 `json:"max_drawdown"`
	MaxWinStreak  int     `json:"max_win_streak"`
	MaxLoseStreak int     `json:"max_lose_streak"`
	IsProfitable  bool    `json:"is_profitable"`
	HitBigWin     bool    `json:"hit_big_win"`
	ActualRTP     float64 `json:"actual_rtp"`
}

// Summary returns a condensed summary of the player's session.
func (p *Player) Summary() PlayerSummary {
	return PlayerSummary{
		ID:            p.ID,
		FinalBalance:  round2(p.CurrentBalance),
		PeakBalance:   round2(p.PeakBalance),
		MinBalance:    round2(p.MinBalance),
		MaxDrawdown:   round4(p.MaxDrawdown),
		MaxWinStreak:  p.MaxWinStreak,
		MaxLoseStreak: p.MaxLoseStreak,
		IsProfitable:  p.IsProfitable(),
		HitBigWin:     p.HitBigWin(),
		ActualRTP:     round4(p.ActualRTP()),
	}
}

// round2 rounds to 2 decimal places.
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// round4 rounds to 4 decimal places.
func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}
