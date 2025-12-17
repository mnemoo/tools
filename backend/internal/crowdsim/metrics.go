package crowdsim

import (
	"math"
	"sort"

	"lutexplorer/internal/common"
)

// CalcPoP calculates the Probability of Profit (players ending with >= initial balance).
func CalcPoP(players []*Player) float64 {
	if len(players) == 0 {
		return 0
	}

	profitable := 0
	for _, p := range players {
		if p.IsProfitable() {
			profitable++
		}
	}

	return round4(float64(profitable) / float64(len(players)))
}

// CalcPoPCurve calculates the PoP at each spin number.
// Returns slice of length spinsPerSession (index 0 = after spin 1).
// Spin 0 (before any play) is excluded as it's meaningless (everyone at initial balance).
func CalcPoPCurve(players []*Player, spinsPerSession int, initialBalance float64) []float64 {
	if len(players) == 0 {
		return nil
	}

	// Check if any player has history
	hasHistory := false
	for _, p := range players {
		if p.BalanceHistory != nil && len(p.BalanceHistory) > 0 {
			hasHistory = true
			break
		}
	}

	if !hasHistory {
		// No history available, return nil
		return nil
	}

	// Start from spin 1 (after first bet), skip spin 0 (initial state)
	curve := make([]float64, spinsPerSession)
	playerCount := float64(len(players))

	for spin := 1; spin <= spinsPerSession; spin++ {
		inProfit := 0
		for _, p := range players {
			if p.BalanceHistory != nil && spin < len(p.BalanceHistory) {
				if p.BalanceHistory[spin] >= initialBalance {
					inProfit++
				}
			}
		}
		curve[spin-1] = round4(float64(inProfit) / playerCount)
	}

	return curve
}

// BalanceCurvePoint represents a single point on the balance curve.
type BalanceCurvePoint struct {
	Spin   int     `json:"spin"`
	Avg    float64 `json:"avg"`
	Median float64 `json:"median"`
	P5     float64 `json:"p5"`  // 5th percentile (worst players)
	P95    float64 `json:"p95"` // 95th percentile (best players)
}

// CalcBalanceCurve calculates average, median, and percentile balances at each spin.
// Returns sampled points to avoid huge response (every N spins).
func CalcBalanceCurve(players []*Player, spinsPerSession int) []BalanceCurvePoint {
	if len(players) == 0 {
		return nil
	}

	// Check if any player has history
	hasHistory := false
	for _, p := range players {
		if p.BalanceHistory != nil && len(p.BalanceHistory) > 0 {
			hasHistory = true
			break
		}
	}

	if !hasHistory {
		return nil
	}

	// Sample every N spins to keep response size manageable
	// For 300 spins, sample ~30 points
	sampleInterval := spinsPerSession / 30
	if sampleInterval < 1 {
		sampleInterval = 1
	}

	points := make([]BalanceCurvePoint, 0, 32)
	balances := make([]float64, len(players))

	for spin := 0; spin <= spinsPerSession; spin += sampleInterval {
		// Collect balances at this spin
		validCount := 0
		for i, p := range players {
			if p.BalanceHistory != nil && spin < len(p.BalanceHistory) {
				balances[i] = p.BalanceHistory[spin]
				validCount++
			}
		}

		if validCount == 0 {
			continue
		}

		// Calculate stats
		sorted := make([]float64, validCount)
		copy(sorted, balances[:validCount])
		sort.Float64s(sorted)

		var sum float64
		for _, b := range sorted {
			sum += b
		}

		points = append(points, BalanceCurvePoint{
			Spin:   spin,
			Avg:    round2(sum / float64(validCount)),
			Median: round2(percentile(sorted, 50)),
			P5:     round2(percentile(sorted, 5)),
			P95:    round2(percentile(sorted, 95)),
		})
	}

	// Always include the final spin if not already included
	lastSpin := spinsPerSession
	if len(points) > 0 && points[len(points)-1].Spin != lastSpin {
		validCount := 0
		for i, p := range players {
			if p.BalanceHistory != nil && lastSpin < len(p.BalanceHistory) {
				balances[i] = p.BalanceHistory[lastSpin]
				validCount++
			}
		}

		if validCount > 0 {
			sorted := make([]float64, validCount)
			copy(sorted, balances[:validCount])
			sort.Float64s(sorted)

			var sum float64
			for _, b := range sorted {
				sum += b
			}

			points = append(points, BalanceCurvePoint{
				Spin:   lastSpin,
				Avg:    round2(sum / float64(validCount)),
				Median: round2(percentile(sorted, 50)),
				P5:     round2(percentile(sorted, 5)),
				P95:    round2(percentile(sorted, 95)),
			})
		}
	}

	return points
}

// BalanceStats holds balance distribution statistics.
type BalanceStats struct {
	Mean         float64            `json:"mean"`
	Median       float64            `json:"median"`
	StdDev       float64            `json:"std_dev"`
	Min          float64            `json:"min"`
	Max          float64            `json:"max"`
	Percentiles  map[string]float64 `json:"percentiles"`
	Distribution []BalanceBucket    `json:"distribution,omitempty"`
}

// BalanceBucket represents a balance range for histogram.
type BalanceBucket struct {
	RangeStart float64 `json:"range_start"`
	RangeEnd   float64 `json:"range_end"`
	Count      int     `json:"count"`
	Percent    float64 `json:"percent"`
}

// CalcBalanceStats calculates balance distribution statistics.
func CalcBalanceStats(players []*Player) BalanceStats {
	if len(players) == 0 {
		return BalanceStats{}
	}

	// Collect final balances
	balances := make([]float64, len(players))
	for i, p := range players {
		balances[i] = p.CurrentBalance
	}

	// Sort for percentiles
	sorted := make([]float64, len(balances))
	copy(sorted, balances)
	sort.Float64s(sorted)

	// Calculate mean
	var sum float64
	for _, b := range balances {
		sum += b
	}
	mean := sum / float64(len(balances))

	// Calculate std dev
	var sumSquares float64
	for _, b := range balances {
		diff := b - mean
		sumSquares += diff * diff
	}
	stdDev := math.Sqrt(sumSquares / float64(len(balances)))

	// Calculate percentiles
	percentiles := map[string]float64{
		"5":  round2(percentile(sorted, 5)),
		"10": round2(percentile(sorted, 10)),
		"25": round2(percentile(sorted, 25)),
		"50": round2(percentile(sorted, 50)),
		"75": round2(percentile(sorted, 75)),
		"90": round2(percentile(sorted, 90)),
		"95": round2(percentile(sorted, 95)),
	}

	// Build distribution buckets
	distribution := buildBalanceDistribution(sorted)

	return BalanceStats{
		Mean:         round2(mean),
		Median:       round2(percentile(sorted, 50)),
		StdDev:       round2(stdDev),
		Min:          round2(sorted[0]),
		Max:          round2(sorted[len(sorted)-1]),
		Percentiles:  percentiles,
		Distribution: distribution,
	}
}

// buildBalanceDistribution creates histogram buckets.
func buildBalanceDistribution(sorted []float64) []BalanceBucket {
	if len(sorted) == 0 {
		return nil
	}

	// Fixed buckets relative to 100 initial balance
	ranges := common.DefaultBalanceRanges()

	buckets := make([]BalanceBucket, len(ranges))
	total := float64(len(sorted))

	for i, r := range ranges {
		count := 0
		for _, v := range sorted {
			if v >= r.Start && v < r.End {
				count++
			}
		}
		buckets[i] = BalanceBucket{
			RangeStart: r.Start,
			RangeEnd:   r.End,
			Count:      count,
			Percent:    round2(float64(count) / total * 100),
		}
	}

	return buckets
}

// percentile calculates the p-th percentile of sorted data.
func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	idx := float64(p) / 100.0 * float64(len(sorted)-1)
	lower := int(idx)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// PeakStats holds peak balance statistics.
type PeakStats struct {
	AvgPeak    float64 `json:"avg_peak"`
	MedianPeak float64 `json:"median_peak"`
	MaxPeak    float64 `json:"max_peak"`
	MinPeak    float64 `json:"min_peak"`
}

// CalcPeakStats calculates peak balance statistics.
func CalcPeakStats(players []*Player) PeakStats {
	if len(players) == 0 {
		return PeakStats{}
	}

	peaks := make([]float64, len(players))
	for i, p := range players {
		peaks[i] = p.PeakBalance
	}

	sort.Float64s(peaks)

	var sum float64
	for _, v := range peaks {
		sum += v
	}

	return PeakStats{
		AvgPeak:    round2(sum / float64(len(peaks))),
		MedianPeak: round2(percentile(peaks, 50)),
		MaxPeak:    round2(peaks[len(peaks)-1]),
		MinPeak:    round2(peaks[0]),
	}
}

// DrawdownStats holds drawdown analysis results.
type DrawdownStats struct {
	AvgMaxDrawdown      float64 `json:"avg_max_drawdown"`
	MedianMaxDrawdown   float64 `json:"median_max_drawdown"`
	PlayersBelow50Pct   int     `json:"players_below_50pct"`
	PlayersBelow90Pct   int     `json:"players_below_90pct"`
	PercentBelow50      float64 `json:"percent_below_50"`
	PercentBelow90      float64 `json:"percent_below_90"`
	MaxDrawdownObserved float64 `json:"max_drawdown_observed"`
}

// CalcDrawdownStats calculates drawdown analysis results.
func CalcDrawdownStats(players []*Player) DrawdownStats {
	if len(players) == 0 {
		return DrawdownStats{}
	}

	drawdowns := make([]float64, len(players))
	below50 := 0
	below90 := 0

	for i, p := range players {
		drawdowns[i] = p.MaxDrawdown
		if p.MaxDrawdown > 0.5 {
			below50++
		}
		if p.MaxDrawdown > 0.9 {
			below90++
		}
	}

	sort.Float64s(drawdowns)

	var sum float64
	for _, d := range drawdowns {
		sum += d
	}

	total := float64(len(players))

	return DrawdownStats{
		AvgMaxDrawdown:      round4(sum / total),
		MedianMaxDrawdown:   round4(percentile(drawdowns, 50)),
		PlayersBelow50Pct:   below50,
		PlayersBelow90Pct:   below90,
		PercentBelow50:      round2(float64(below50) / total * 100),
		PercentBelow90:      round2(float64(below90) / total * 100),
		MaxDrawdownObserved: round4(drawdowns[len(drawdowns)-1]),
	}
}

// DangerStats holds near-bankrupt event statistics.
type DangerStats struct {
	TotalDangerEvents int     `json:"total_danger_events"`
	PlayersWithDanger int     `json:"players_with_danger"`
	AvgDangerEvents   float64 `json:"avg_danger_events"`
	PercentWithDanger float64 `json:"percent_with_danger"`
}

// CalcDangerStats calculates danger event statistics.
func CalcDangerStats(players []*Player) DangerStats {
	if len(players) == 0 {
		return DangerStats{}
	}

	total := 0
	withDanger := 0

	for _, p := range players {
		total += p.DangerEvents
		if p.DangerEvents > 0 {
			withDanger++
		}
	}

	count := float64(len(players))

	return DangerStats{
		TotalDangerEvents: total,
		PlayersWithDanger: withDanger,
		AvgDangerEvents:   round2(float64(total) / count),
		PercentWithDanger: round2(float64(withDanger) / count * 100),
	}
}

// StreakStats holds winning/losing streak statistics.
type StreakStats struct {
	AvgWinStreak  float64 `json:"avg_win_streak"`
	MaxWinStreak  int     `json:"max_win_streak"`
	AvgLoseStreak float64 `json:"avg_lose_streak"`
	MaxLoseStreak int     `json:"max_lose_streak"`
}

// CalcStreakStats calculates streak statistics.
func CalcStreakStats(players []*Player) StreakStats {
	if len(players) == 0 {
		return StreakStats{}
	}

	var sumWin, sumLose float64
	maxWin, maxLose := 0, 0

	for _, p := range players {
		sumWin += float64(p.MaxWinStreak)
		sumLose += float64(p.MaxLoseStreak)
		if p.MaxWinStreak > maxWin {
			maxWin = p.MaxWinStreak
		}
		if p.MaxLoseStreak > maxLose {
			maxLose = p.MaxLoseStreak
		}
	}

	count := float64(len(players))

	return StreakStats{
		AvgWinStreak:  round2(sumWin / count),
		MaxWinStreak:  maxWin,
		AvgLoseStreak: round2(sumLose / count),
		MaxLoseStreak: maxLose,
	}
}

// BigWinStats holds time-to-first-big-win statistics.
type BigWinStats struct {
	AvgSpinsToFirst    float64 `json:"avg_spins_to_first"`
	MedianSpinsToFirst float64 `json:"median_spins_to_first"`
	PlayersNeverHit    int     `json:"players_never_hit"`
	PercentNeverHit    float64 `json:"percent_never_hit"`
	PlayersHit         int     `json:"players_hit"`
	PercentHit         float64 `json:"percent_hit"`
}

// CalcBigWinStats calculates big win statistics.
func CalcBigWinStats(players []*Player) BigWinStats {
	if len(players) == 0 {
		return BigWinStats{}
	}

	spinsToHit := make([]float64, 0, len(players))
	neverHit := 0

	for _, p := range players {
		if p.FirstBigWinSpin >= 0 {
			spinsToHit = append(spinsToHit, float64(p.FirstBigWinSpin+1)) // 1-indexed
		} else {
			neverHit++
		}
	}

	total := float64(len(players))
	hit := len(spinsToHit)

	stats := BigWinStats{
		PlayersNeverHit: neverHit,
		PercentNeverHit: round2(float64(neverHit) / total * 100),
		PlayersHit:      hit,
		PercentHit:      round2(float64(hit) / total * 100),
	}

	if hit > 0 {
		sort.Float64s(spinsToHit)
		var sum float64
		for _, s := range spinsToHit {
			sum += s
		}
		stats.AvgSpinsToFirst = round2(sum / float64(hit))
		stats.MedianSpinsToFirst = round2(percentile(spinsToHit, 50))
	}

	return stats
}

// VolatilityProfile represents volatility classification.
type VolatilityProfile string

const (
	VolatilityLow    VolatilityProfile = "low"
	VolatilityMedium VolatilityProfile = "medium"
	VolatilityHigh   VolatilityProfile = "high"
)

// ClassifyVolatility determines volatility profile based on metrics.
func ClassifyVolatility(pop float64, balanceStats BalanceStats, peakStats PeakStats, initialBalance float64) VolatilityProfile {
	// Low volatility criteria:
	// - PoP >= 0.40
	// - Low std deviation (< 50% of initial)
	// - Max peak not extremely high (< 3x initial)
	if pop >= 0.40 && balanceStats.StdDev < initialBalance*0.5 && peakStats.MaxPeak < initialBalance*3 {
		return VolatilityLow
	}

	// High volatility criteria:
	// - PoP < 0.20
	// - High std deviation (> 100% of initial)
	// - Max peak very high (> 5x initial)
	if pop < 0.20 || balanceStats.StdDev > initialBalance || peakStats.MaxPeak > initialBalance*5 {
		return VolatilityHigh
	}

	return VolatilityMedium
}

// CalcCompositeScore calculates a weighted composite score.
func CalcCompositeScore(result *SimResult, weights RankingWeights, initialBalance float64) float64 {
	score := 0.0

	// Profit probability contribution (higher is better)
	score += result.FinalPoP * weights.ProfitWeight

	// Safety contribution (lower drawdown is better)
	score += (1 - result.DrawdownStats.AvgMaxDrawdown) * weights.SafetyWeight

	// Excitement contribution (higher peak relative to initial is better)
	peakRatio := result.PeakStats.AvgPeak / initialBalance
	if peakRatio > 3 {
		peakRatio = 3 // Cap at 3x for scoring
	}
	score += (peakRatio / 3) * weights.ExcitementWeight

	// Frustration penalty (longer lose streaks are worse)
	loseStreakPenalty := float64(result.StreakStats.AvgLoseStreak) / 20.0 // Normalize to ~0-1
	if loseStreakPenalty > 1 {
		loseStreakPenalty = 1
	}
	score -= loseStreakPenalty * weights.FrustrationPen

	return round4(score)
}
