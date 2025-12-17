package crowdsim

// ModeInfo contains information about the mode type and cost
type ModeInfo struct {
	Cost        float64 `json:"cost"`
	IsBonusMode bool    `json:"is_bonus_mode"`
	Note        string  `json:"note"`
}

// SimResult holds complete simulation results.
type SimResult struct {
	// Metadata
	Mode       string    `json:"mode"`
	ModeInfo   ModeInfo  `json:"mode_info"`
	Config     SimConfig `json:"config"`
	DurationMs int64     `json:"duration_ms"`

	// RTP Validation
	TheoreticalRTP float64 `json:"theoretical_rtp"`
	ActualRTP      float64 `json:"actual_rtp"`
	RTPDeviation   float64 `json:"rtp_deviation"`

	// Primary Metrics
	FinalPoP     float64             `json:"final_pop"`
	PoPCurve     []float64           `json:"pop_curve,omitempty"`
	BalanceCurve []BalanceCurvePoint `json:"balance_curve,omitempty"`
	BalanceStats BalanceStats        `json:"balance_stats"`

	// Secondary Metrics
	PeakStats     PeakStats     `json:"peak_stats"`
	DrawdownStats DrawdownStats `json:"drawdown_stats"`
	DangerStats   DangerStats   `json:"danger_stats"`
	StreakStats   StreakStats   `json:"streak_stats"`
	BigWinStats   BigWinStats   `json:"big_win_stats"`

	// Classification
	VolatilityProfile VolatilityProfile `json:"volatility_profile"`
	CompositeScore    float64           `json:"composite_score"`

	// Detailed Data (when not in streaming mode and player count <= 1000)
	PlayerSummaries []PlayerSummary `json:"player_summaries,omitempty"`
}

// CompareRequest is the request body for comparing multiple modes.
type CompareRequest struct {
	Modes  []string  `json:"modes"`
	Config SimConfig `json:"config"`
}

// CompareResult holds comparison results for multiple modes.
type CompareResult struct {
	Results []SimResult    `json:"results"`
	Ranking []RankedResult `json:"ranking"`
}

// RankedResult holds a ranked simulation result.
type RankedResult struct {
	Mode  string  `json:"mode"`
	Score float64 `json:"score"`
	Rank  int     `json:"rank"`
}

// RankResults sorts results by composite score and assigns ranks.
func RankResults(results []SimResult) []RankedResult {
	ranked := make([]RankedResult, len(results))

	for i, r := range results {
		ranked[i] = RankedResult{
			Mode:  r.Mode,
			Score: r.CompositeScore,
		}
	}

	// Sort by score descending
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].Score > ranked[i].Score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	// Assign ranks
	for i := range ranked {
		ranked[i].Rank = i + 1
	}

	return ranked
}

// ValidationResult holds RTP validation information.
type ValidationResult struct {
	Mode           string  `json:"mode"`
	TheoreticalRTP float64 `json:"theoretical_rtp"`
	ActualRTP      float64 `json:"actual_rtp"`
	Deviation      float64 `json:"deviation"`
	DeviationPct   float64 `json:"deviation_pct"`
	IsValid        bool    `json:"is_valid"`
	Tolerance      float64 `json:"tolerance"`
}

// ValidateRTP checks if simulation RTP matches theoretical within tolerance.
func ValidateRTP(result *SimResult, tolerancePct float64) ValidationResult {
	deviation := result.ActualRTP - result.TheoreticalRTP
	deviationPct := 0.0
	if result.TheoreticalRTP != 0 {
		deviationPct = (deviation / result.TheoreticalRTP) * 100
	}

	return ValidationResult{
		Mode:           result.Mode,
		TheoreticalRTP: result.TheoreticalRTP,
		ActualRTP:      result.ActualRTP,
		Deviation:      round4(deviation),
		DeviationPct:   round2(deviationPct),
		IsValid:        deviationPct <= tolerancePct && deviationPct >= -tolerancePct,
		Tolerance:      tolerancePct,
	}
}

// VolatilityThresholds defines acceptance criteria for different volatility profiles.
type VolatilityThresholds struct {
	Name               string  `json:"name"`
	MinPoP             float64 `json:"min_pop"`
	MaxPoP             float64 `json:"max_pop"`
	MaxAvgDrawdown     float64 `json:"max_avg_drawdown"`
	MaxPercentBelow90  float64 `json:"max_percent_below_90"`
	MaxAvgLoseStreak   float64 `json:"max_avg_lose_streak"`
	MinPeakRatio       float64 `json:"min_peak_ratio"` // Peak / Initial
	MaxSpinsToBigWin   float64 `json:"max_spins_to_big_win"`
	MaxPercentNeverHit float64 `json:"max_percent_never_hit"`
}

// GetVolatilityThresholds returns acceptance criteria for volatility profiles.
func GetVolatilityThresholds() map[VolatilityProfile]VolatilityThresholds {
	return map[VolatilityProfile]VolatilityThresholds{
		VolatilityLow: {
			Name:               "Low Volatility (Casual Players)",
			MinPoP:             0.40,
			MaxPoP:             1.0,
			MaxAvgDrawdown:     0.60,
			MaxPercentBelow90:  10,
			MaxAvgLoseStreak:   8,
			MinPeakRatio:       1.0,
			MaxSpinsToBigWin:   0, // N/A for low volatility
			MaxPercentNeverHit: 0, // N/A
		},
		VolatilityMedium: {
			Name:               "Medium Volatility (Balanced)",
			MinPoP:             0.25,
			MaxPoP:             0.40,
			MaxAvgDrawdown:     0.75,
			MaxPercentBelow90:  25,
			MaxAvgLoseStreak:   12,
			MinPeakRatio:       1.5,
			MaxSpinsToBigWin:   100,
			MaxPercentNeverHit: 50,
		},
		VolatilityHigh: {
			Name:               "High Volatility (Streamers/High Rollers)",
			MinPoP:             0.10,
			MaxPoP:             0.25,
			MaxAvgDrawdown:     1.0,
			MaxPercentBelow90:  50,
			MaxAvgLoseStreak:   20,
			MinPeakRatio:       3.0,
			MaxSpinsToBigWin:   50,
			MaxPercentNeverHit: 30,
		},
	}
}

// CheckVolatilityCompliance checks if result meets volatility profile criteria.
func CheckVolatilityCompliance(result *SimResult, profile VolatilityProfile, initialBalance float64) map[string]bool {
	thresholds := GetVolatilityThresholds()[profile]
	checks := make(map[string]bool)

	checks["pop_in_range"] = result.FinalPoP >= thresholds.MinPoP && result.FinalPoP <= thresholds.MaxPoP
	checks["drawdown_ok"] = result.DrawdownStats.AvgMaxDrawdown <= thresholds.MaxAvgDrawdown
	checks["below_90_ok"] = result.DrawdownStats.PercentBelow90 <= thresholds.MaxPercentBelow90
	checks["lose_streak_ok"] = result.StreakStats.AvgLoseStreak <= thresholds.MaxAvgLoseStreak

	peakRatio := result.PeakStats.AvgPeak / initialBalance
	checks["peak_ratio_ok"] = peakRatio >= thresholds.MinPeakRatio

	if thresholds.MaxSpinsToBigWin > 0 {
		checks["big_win_timing_ok"] = result.BigWinStats.AvgSpinsToFirst <= thresholds.MaxSpinsToBigWin ||
			result.BigWinStats.AvgSpinsToFirst == 0
	}

	if thresholds.MaxPercentNeverHit > 0 {
		checks["big_win_rate_ok"] = result.BigWinStats.PercentNeverHit <= thresholds.MaxPercentNeverHit
	}

	return checks
}
