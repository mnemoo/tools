// Package lut provides compliance checking for LUT tables.
package lut

import (
	"fmt"

	"stakergs"
)

// ComplianceCheckID identifies a specific compliance check.
type ComplianceCheckID string

const (
	CheckRTPRange          ComplianceCheckID = "rtp_range"
	CheckRTPVariation      ComplianceCheckID = "rtp_variation"
	CheckMaxWinAchievable  ComplianceCheckID = "max_win_achievable"
	CheckHitRateReasonable ComplianceCheckID = "hit_rate_reasonable"
	CheckPayoutGaps        ComplianceCheckID = "payout_gaps"
	CheckUniquePayouts     ComplianceCheckID = "unique_payouts"
	CheckSimulationDiversity ComplianceCheckID = "simulation_diversity"
	CheckZeroPayoutRate    ComplianceCheckID = "zero_payout_rate"
	CheckVolatility        ComplianceCheckID = "volatility"
)

// ComplianceCheck represents a single compliance check result.
type ComplianceCheck struct {
	ID             ComplianceCheckID `json:"id"`
	NameKey        string            `json:"nameKey"`
	DescriptionKey string            `json:"descriptionKey"`
	Passed         bool              `json:"passed"`
	Value          string            `json:"value"`
	Expected       string            `json:"expected"`
	ReasonKey      string            `json:"reasonKey,omitempty"`
	Severity       string            `json:"severity"` // "error", "warning", "info"
	Details        interface{}       `json:"details,omitempty"`
}

// ComplianceResult contains all compliance check results for a mode.
type ComplianceResult struct {
	Mode         string             `json:"mode"`
	Passed       bool               `json:"passed"`
	PassedCount  int                `json:"passed_count"`
	FailedCount  int                `json:"failed_count"`
	WarningCount int                `json:"warning_count"`
	Checks       []ComplianceCheck  `json:"checks"`
	Summary      ComplianceSummary  `json:"summary"`
}

// ComplianceSummary contains summary statistics used for compliance checks.
type ComplianceSummary struct {
	RTP               float64 `json:"rtp"`
	HitRate           float64 `json:"hit_rate"`
	MaxPayout         float64 `json:"max_payout"`
	MaxPayoutHitRate  float64 `json:"max_payout_hit_rate"`
	TotalOutcomes     int     `json:"total_outcomes"`
	UniquePayouts     int     `json:"unique_payouts"`
	ZeroPayoutRate    float64 `json:"zero_payout_rate"`
	Volatility        float64 `json:"volatility"`
	MostFrequentProb  float64 `json:"most_frequent_probability"`
}

// AllModesComplianceResult contains compliance results for all modes.
type AllModesComplianceResult struct {
	AllPassed   bool                        `json:"all_passed"`
	ModeResults map[string]*ComplianceResult `json:"mode_results"`
	GlobalChecks []ComplianceCheck           `json:"global_checks"`
}

// ComplianceChecker performs compliance checks on LUT tables.
type ComplianceChecker struct {
	analyzer *Analyzer
}

// NewComplianceChecker creates a new compliance checker.
func NewComplianceChecker() *ComplianceChecker {
	return &ComplianceChecker{
		analyzer: NewAnalyzer(),
	}
}

// CheckMode performs all compliance checks on a single mode.
func (c *ComplianceChecker) CheckMode(lut *stakergs.LookupTable) *ComplianceResult {
	stats := c.analyzer.Analyze(lut)
	totalWeight := lut.TotalWeight()

	result := &ComplianceResult{
		Mode:   lut.Mode,
		Checks: make([]ComplianceCheck, 0),
		Summary: ComplianceSummary{
			RTP:           stats.RTP,
			HitRate:       stats.HitRate,
			MaxPayout:     stats.MaxPayout,
			TotalOutcomes: stats.TotalOutcomes,
			ZeroPayoutRate: stats.ZeroPayoutRate,
			Volatility:    stats.Volatility,
		},
	}

	// Calculate additional summary values
	result.Summary.UniquePayouts = c.countUniquePayouts(lut)
	result.Summary.MaxPayoutHitRate = c.calculateMaxPayoutHitRate(lut, totalWeight)
	result.Summary.MostFrequentProb, _ = c.calculateMostFrequentProbability(lut, totalWeight)

	// Run all checks
	result.Checks = append(result.Checks, c.checkRTPRange(stats))
	result.Checks = append(result.Checks, c.checkMaxWinAchievable(lut, totalWeight, stats))
	result.Checks = append(result.Checks, c.checkHitRateReasonable(lut, stats))
	result.Checks = append(result.Checks, c.checkPayoutGaps(lut, stats))
	result.Checks = append(result.Checks, c.checkUniquePayouts(lut))
	result.Checks = append(result.Checks, c.checkSimulationDiversity(lut, totalWeight))
	result.Checks = append(result.Checks, c.checkZeroPayoutRate(stats))
	result.Checks = append(result.Checks, c.checkVolatility(stats))

	// Calculate totals
	for _, check := range result.Checks {
		if check.Passed {
			result.PassedCount++
		} else if check.Severity == "warning" {
			result.WarningCount++
		} else {
			result.FailedCount++
		}
	}

	result.Passed = result.FailedCount == 0

	return result
}

// CheckAllModes performs compliance checks on all modes and cross-mode checks.
func (c *ComplianceChecker) CheckAllModes(tables map[string]*stakergs.LookupTable) *AllModesComplianceResult {
	result := &AllModesComplianceResult{
		ModeResults:  make(map[string]*ComplianceResult),
		GlobalChecks: make([]ComplianceCheck, 0),
		AllPassed:    true,
	}

	// Compute base RTP first (needed for per-mode checks)
	baseRTP, baseModeName := c.findBaseRTP(tables)

	// Check each mode individually
	for mode, lut := range tables {
		modeResult := c.CheckMode(lut)

		// Add per-mode RTP variation check if we have multiple modes
		if len(tables) > 1 {
			rtpCheck := c.checkModeRTPVariation(lut, baseRTP, baseModeName)
			modeResult.Checks = append(modeResult.Checks, rtpCheck)

			// Update counts
			if rtpCheck.Passed {
				modeResult.PassedCount++
			} else if rtpCheck.Severity == "warning" {
				modeResult.WarningCount++
			} else {
				modeResult.FailedCount++
				modeResult.Passed = false
			}
		}

		result.ModeResults[mode] = modeResult
		if !modeResult.Passed {
			result.AllPassed = false
		}
	}

	// Global cross-mode RTP variation check (summary)
	if len(tables) > 1 {
		rtpCheck := c.checkRTPVariationGlobal(tables, baseRTP, baseModeName)
		result.GlobalChecks = append(result.GlobalChecks, rtpCheck)
		if !rtpCheck.Passed && rtpCheck.Severity == "error" {
			result.AllPassed = false
		}
	}

	return result
}

// findBaseRTP finds the base RTP for cross-mode comparison.
// Prefers mode named "base", otherwise uses mode with highest RTP.
func (c *ComplianceChecker) findBaseRTP(tables map[string]*stakergs.LookupTable) (float64, string) {
	// First, try to find a mode named "base"
	if lut, ok := tables["base"]; ok {
		return lut.RTP(), "base"
	}

	// No "base" mode found, use mode with highest RTP
	var baseRTP float64
	var baseModeName string
	for mode, lut := range tables {
		rtp := lut.RTP()
		if rtp > baseRTP || baseModeName == "" {
			baseRTP = rtp
			baseModeName = mode
		}
	}

	return baseRTP, baseModeName
}

// checkModeRTPVariation checks if a single mode's RTP is within allowed range of base RTP.
func (c *ComplianceChecker) checkModeRTPVariation(lut *stakergs.LookupTable, baseRTP float64, baseModeName string) ComplianceCheck {
	maxVariation := 0.005 // 0.5%
	minAllowed := baseRTP - maxVariation
	maxAllowed := baseRTP + maxVariation

	modeRTP := lut.RTP()
	deviation := modeRTP - baseRTP
	if deviation < 0 {
		deviation = -deviation
	}

	isInRange := modeRTP >= minAllowed && modeRTP <= maxAllowed

	check := ComplianceCheck{
		ID:             CheckRTPVariation,
		NameKey:        "compliance.checks.rtpVariation.name",
		DescriptionKey: "compliance.checks.rtpVariation.description",
		Expected:       fmt.Sprintf("%.2f%% - %.2f%%", minAllowed*100, maxAllowed*100),
		Value:          fmt.Sprintf("%.2f%% (deviation: %.2f%%)", modeRTP*100, deviation*100),
		Severity:       "error",
		Details: map[string]interface{}{
			"base_mode":   baseModeName,
			"base_rtp":    baseRTP,
			"mode_rtp":    modeRTP,
			"deviation":   deviation,
			"min_allowed": minAllowed,
			"max_allowed": maxAllowed,
		},
	}

	if isInRange {
		check.Passed = true
	} else {
		check.Passed = false
		if modeRTP < minAllowed {
			check.ReasonKey = "compliance.checks.rtpVariation.reasonLow"
		} else {
			check.ReasonKey = "compliance.checks.rtpVariation.reasonHigh"
		}
	}

	return check
}

func (c *ComplianceChecker) checkRTPRange(stats *Statistics) ComplianceCheck {
	minRTP := 0.90
	maxRTP := 0.98

	check := ComplianceCheck{
		ID:             CheckRTPRange,
		NameKey:        "compliance.checks.rtpRange.name",
		DescriptionKey: "compliance.checks.rtpRange.description",
		Expected:       fmt.Sprintf("%.1f%% - %.1f%%", minRTP*100, maxRTP*100),
		Value:          fmt.Sprintf("%.2f%%", stats.RTP*100),
		Severity:       "error",
	}

	if stats.RTP >= minRTP && stats.RTP <= maxRTP {
		check.Passed = true
	} else {
		check.Passed = false
		if stats.RTP < minRTP {
			check.ReasonKey = "compliance.checks.rtpRange.reasonLow"
		} else {
			check.ReasonKey = "compliance.checks.rtpRange.reasonHigh"
		}
	}

	return check
}

// checkRTPVariationGlobal creates a global summary of RTP variation across all modes.
func (c *ComplianceChecker) checkRTPVariationGlobal(tables map[string]*stakergs.LookupTable, baseRTP float64, baseModeName string) ComplianceCheck {
	maxVariation := 0.005 // 0.5%
	minAllowed := baseRTP - maxVariation
	maxAllowed := baseRTP + maxVariation

	modeRTPs := make(map[string]float64)
	var outOfRangeModes []string
	var maxDeviation float64

	for mode, lut := range tables {
		rtp := lut.RTP()
		modeRTPs[mode] = rtp

		deviation := rtp - baseRTP
		if deviation < 0 {
			deviation = -deviation
		}
		if deviation > maxDeviation {
			maxDeviation = deviation
		}
		if rtp < minAllowed || rtp > maxAllowed {
			outOfRangeModes = append(outOfRangeModes, mode)
		}
	}

	totalModes := len(tables)
	passedModes := totalModes - len(outOfRangeModes)

	check := ComplianceCheck{
		ID:             CheckRTPVariation,
		NameKey:        "compliance.checks.rtpVariationGlobal.name",
		DescriptionKey: "compliance.checks.rtpVariationGlobal.description",
		Expected:       fmt.Sprintf("%.2f%% - %.2f%%", minAllowed*100, maxAllowed*100),
		Value:          fmt.Sprintf("%d/%d modes passed", passedModes, totalModes),
		Severity:       "error",
		Details: map[string]interface{}{
			"base_mode":     baseModeName,
			"base_rtp":      baseRTP,
			"min_allowed":   minAllowed,
			"max_allowed":   maxAllowed,
			"mode_rtps":     modeRTPs,
			"out_of_range":  outOfRangeModes,
			"max_deviation": maxDeviation,
			"passed_count":  passedModes,
			"failed_count":  len(outOfRangeModes),
		},
	}

	if len(outOfRangeModes) == 0 {
		check.Passed = true
	} else {
		check.Passed = false
		check.ReasonKey = "compliance.checks.rtpVariationGlobal.reason"
	}

	return check
}

func (c *ComplianceChecker) checkMaxWinAchievable(lut *stakergs.LookupTable, totalWeight uint64, stats *Statistics) ComplianceCheck {
	// Max win should be achievable with hit-rate of at least 1 in 20,000,000 for base mode (cost=1)
	// For bonus modes with higher cost, the threshold is adjusted: 20,000,000 / cost
	// Example: bonus with cost=200x -> maxOdds = 20,000,000 / 200 = 100,000
	baseMaxOdds := 20_000_000.0
	cost := lut.Cost
	if cost <= 0 {
		cost = 1.0
	}
	maxOdds := baseMaxOdds / cost

	var maxPayoutWeight uint64
	maxPayout := lut.MaxPayout()
	for _, o := range lut.Outcomes {
		if o.Payout == maxPayout {
			maxPayoutWeight += o.Weight
		}
	}

	actualOdds := float64(totalWeight) / float64(maxPayoutWeight)

	check := ComplianceCheck{
		ID:             CheckMaxWinAchievable,
		NameKey:        "compliance.checks.maxWinAchievable.name",
		DescriptionKey: "compliance.checks.maxWinAchievable.description",
		Expected:       fmt.Sprintf("Odds ≤ 1 in %s", formatLargeNumber(maxOdds)),
		Value:          fmt.Sprintf("1 in %s", formatLargeNumber(actualOdds)),
		Severity:       "error",
		Details: map[string]interface{}{
			"max_payout":        stats.MaxPayout,
			"max_payout_weight": maxPayoutWeight,
			"total_weight":      totalWeight,
			"actual_odds":       actualOdds,
			"mode_cost":         cost,
			"base_max_odds":     baseMaxOdds,
			"adjusted_max_odds": maxOdds,
		},
	}

	if actualOdds <= maxOdds {
		check.Passed = true
	} else {
		check.Passed = false
		check.ReasonKey = "compliance.checks.maxWinAchievable.reason"
	}

	return check
}

func (c *ComplianceChecker) checkHitRateReasonable(lut *stakergs.LookupTable, stats *Statistics) ComplianceCheck {
	// Hit rate check only applies to base modes (cost <= 2x)
	// Bonus modes with higher cost naturally have higher hit rates (often 100%)
	cost := lut.Cost
	if cost <= 0 {
		cost = 1.0
	}

	// Skip check for bonus modes (cost > 2)
	if cost > 2 {
		return ComplianceCheck{
			ID:             CheckHitRateReasonable,
			NameKey:        "compliance.checks.hitRate.name",
			DescriptionKey: "compliance.checks.hitRate.descriptionSkipped",
			Expected:       "N/A (bonus mode)",
			Value:          fmt.Sprintf("%.2f%% (1 in %.2f)", stats.HitRate*100, 1.0/stats.HitRate),
			Severity:       "info",
			Passed:         true,
		}
	}

	// For base modes: hit rate should be between 1 in 3 and 1 in 20
	minHitRate := 0.05 // 1 in 20
	maxHitRate := 0.33 // 1 in 3

	odds := 1.0 / stats.HitRate

	check := ComplianceCheck{
		ID:             CheckHitRateReasonable,
		NameKey:        "compliance.checks.hitRate.name",
		DescriptionKey: "compliance.checks.hitRate.description",
		Expected:       fmt.Sprintf("%.0f%% - %.0f%% (1 in %.0f - 1 in %.0f)", minHitRate*100, maxHitRate*100, 1/maxHitRate, 1/minHitRate),
		Value:          fmt.Sprintf("%.2f%% (1 in %.2f)", stats.HitRate*100, odds),
		Severity:       "warning",
	}

	if stats.HitRate >= minHitRate && stats.HitRate <= maxHitRate {
		check.Passed = true
	} else {
		check.Passed = false
		if stats.HitRate < minHitRate {
			check.ReasonKey = "compliance.checks.hitRate.reasonLow"
		} else {
			check.ReasonKey = "compliance.checks.hitRate.reasonHigh"
		}
	}

	return check
}

func (c *ComplianceChecker) checkPayoutGaps(lut *stakergs.LookupTable, stats *Statistics) ComplianceCheck {
	// Check for significant gaps in payout distribution
	maxPayout := stats.MaxPayout

	// Create buckets for payout ranges
	buckets := []struct {
		start, end float64
		hasPayouts bool
	}{
		{0, 1, false},
		{1, 2, false},
		{2, 5, false},
		{5, 10, false},
		{10, 25, false},
		{25, 50, false},
		{50, 100, false},
		{100, 250, false},
		{250, 500, false},
		{500, 1000, false},
		{1000, 2500, false},
		{2500, 5000, false},
		{5000, maxPayout + 1, false},
	}

	for _, o := range lut.Outcomes {
		payout := float64(o.Payout) / 100.0
		if payout <= 0 {
			continue
		}
		for i := range buckets {
			if payout >= buckets[i].start && payout < buckets[i].end {
				buckets[i].hasPayouts = true
				break
			}
		}
	}

	// Find gaps in populated ranges
	var gaps []string
	inRange := false
	for i, b := range buckets {
		if b.end > maxPayout {
			break
		}
		if b.hasPayouts {
			inRange = true
		} else if inRange && i < len(buckets)-1 && buckets[i+1].hasPayouts {
			gaps = append(gaps, fmt.Sprintf("%.0fx-%.0fx", b.start, b.end))
		}
	}

	check := ComplianceCheck{
		ID:             CheckPayoutGaps,
		NameKey:        "compliance.checks.payoutGaps.name",
		DescriptionKey: "compliance.checks.payoutGaps.description",
		Expected:       "No significant gaps in payout ranges",
		Severity:       "warning",
	}

	if len(gaps) == 0 {
		check.Passed = true
		check.Value = "No gaps detected"
	} else {
		check.Passed = false
		check.Value = fmt.Sprintf("%d gap(s) found", len(gaps))
		check.ReasonKey = "compliance.checks.payoutGaps.reason"
		check.Details = gaps
	}

	return check
}

func (c *ComplianceChecker) checkUniquePayouts(lut *stakergs.LookupTable) ComplianceCheck {
	// For slot-type games, should have reasonable number of unique payout values
	minUnique := 10

	uniquePayouts := c.countUniquePayouts(lut)

	check := ComplianceCheck{
		ID:             CheckUniquePayouts,
		NameKey:        "compliance.checks.uniquePayouts.name",
		DescriptionKey: "compliance.checks.uniquePayouts.description",
		Expected:       fmt.Sprintf("≥ %d unique values", minUnique),
		Value:          fmt.Sprintf("%d unique values", uniquePayouts),
		Severity:       "warning",
	}

	if uniquePayouts >= minUnique {
		check.Passed = true
	} else {
		check.Passed = false
		check.ReasonKey = "compliance.checks.uniquePayouts.reason"
	}

	return check
}

func (c *ComplianceChecker) checkSimulationDiversity(lut *stakergs.LookupTable, totalWeight uint64) ComplianceCheck {
	// No single result should be so frequent that it appears multiple times in a typical session
	// With 100,000 simulations, a single result shouldn't exceed ~1% probability
	maxSingleProb := 0.01 // 1%

	mostFreqProb, _ := c.calculateMostFrequentProbability(lut, totalWeight)

	check := ComplianceCheck{
		ID:             CheckSimulationDiversity,
		NameKey:        "compliance.checks.simulationDiversity.name",
		DescriptionKey: "compliance.checks.simulationDiversity.description",
		Expected:       fmt.Sprintf("Most frequent outcome < %.1f%%", maxSingleProb*100),
		Value:          fmt.Sprintf("%.2f%%", mostFreqProb*100),
		Severity:       "warning",
	}

	if mostFreqProb <= maxSingleProb {
		check.Passed = true
	} else {
		check.Passed = false
		check.ReasonKey = "compliance.checks.simulationDiversity.reason"
	}

	return check
}

func (c *ComplianceChecker) checkZeroPayoutRate(stats *Statistics) ComplianceCheck {
	// Non-paying results shouldn't exceed 90%
	maxZeroRate := 0.90

	check := ComplianceCheck{
		ID:             CheckZeroPayoutRate,
		NameKey:        "compliance.checks.zeroPayoutRate.name",
		DescriptionKey: "compliance.checks.zeroPayoutRate.description",
		Expected:       fmt.Sprintf("Non-paying ≤ %.0f%%", maxZeroRate*100),
		Value:          fmt.Sprintf("%.2f%% non-paying", stats.ZeroPayoutRate*100),
		Severity:       "error",
	}

	if stats.ZeroPayoutRate <= maxZeroRate {
		check.Passed = true
	} else {
		check.Passed = false
		check.ReasonKey = "compliance.checks.zeroPayoutRate.reason"
	}

	return check
}

func (c *ComplianceChecker) checkVolatility(stats *Statistics) ComplianceCheck {
	// Volatility check - standard deviation should be within industry norms
	// This is more informational
	maxVolatility := 50.0 // Very high volatility threshold

	check := ComplianceCheck{
		ID:             CheckVolatility,
		NameKey:        "compliance.checks.volatility.name",
		DescriptionKey: "compliance.checks.volatility.description",
		Expected:       fmt.Sprintf("Volatility < %.0f", maxVolatility),
		Value:          fmt.Sprintf("%.2f", stats.Volatility),
		Severity:       "info",
	}

	if stats.Volatility < maxVolatility {
		check.Passed = true
	} else {
		check.Passed = false
		check.ReasonKey = "compliance.checks.volatility.reason"
	}

	return check
}

// Helper functions

func (c *ComplianceChecker) countUniquePayouts(lut *stakergs.LookupTable) int {
	payouts := make(map[uint]struct{})
	for _, o := range lut.Outcomes {
		payouts[o.Payout] = struct{}{}
	}
	return len(payouts)
}

func (c *ComplianceChecker) calculateMaxPayoutHitRate(lut *stakergs.LookupTable, totalWeight uint64) float64 {
	maxPayout := lut.MaxPayout()
	var maxWeight uint64
	for _, o := range lut.Outcomes {
		if o.Payout == maxPayout {
			maxWeight += o.Weight
		}
	}
	if totalWeight == 0 {
		return 0
	}
	return round4(float64(maxWeight) / float64(totalWeight))
}

func (c *ComplianceChecker) calculateMostFrequentProbability(lut *stakergs.LookupTable, totalWeight uint64) (float64, float64) {
	if totalWeight == 0 {
		return 0, 0
	}

	// Aggregate weights per payout while tracking top/second and zero payout weight.
	payoutWeights := make(map[uint]uint64, len(lut.Outcomes))
	var topWeight, secondWeight uint64
	var topPayout uint
	var zeroWeight uint64

	for _, o := range lut.Outcomes {
		p := o.Payout
		w := payoutWeights[p] + o.Weight
		payoutWeights[p] = w

		if p == 0 {
			zeroWeight = w
		}

		if p == topPayout {
			topWeight = w
		} else if w > topWeight {
			secondWeight = topWeight
			topWeight = w
			topPayout = p
		} else if w > secondWeight {
			secondWeight = w
		}
	}

	var chosen uint64
	if topPayout == 0 {
		chosen = secondWeight
	} else {
		chosen = topWeight
	}

	var mostFreqProb, zeroProb float64
	if chosen > 0 {
		mostFreqProb = round4(float64(chosen) / float64(totalWeight))
	}
	if zeroWeight > 0 {
		zeroProb = round4(float64(zeroWeight) / float64(totalWeight))
	}

	return mostFreqProb, zeroProb
}

func formatLargeNumber(n float64) string {
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.2fB", n/1_000_000_000)
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", n/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.2fK", n/1_000)
	}
	return fmt.Sprintf("%.0f", n)
}

// PayoutGapDetail contains details about payout distribution gaps.
type PayoutGapDetail struct {
	Range       string  `json:"range"`
	HasPayouts  bool    `json:"has_payouts"`
	TotalWeight uint64  `json:"total_weight"`
	Probability float64 `json:"probability"`
}

// GetPayoutRangeAnalysis returns detailed analysis of payout ranges.
func (c *ComplianceChecker) GetPayoutRangeAnalysis(lut *stakergs.LookupTable) []PayoutGapDetail {
	totalWeight := lut.TotalWeight()
	maxPayout := float64(lut.MaxPayout()) / 100.0

	ranges := []struct {
		start, end float64
		label      string
	}{
		{0, 0.01, "0 (No win)"},
		{0.01, 1, "0.01x - 1x"},
		{1, 2, "1x - 2x"},
		{2, 5, "2x - 5x"},
		{5, 10, "5x - 10x"},
		{10, 25, "10x - 25x"},
		{25, 50, "25x - 50x"},
		{50, 100, "50x - 100x"},
		{100, 250, "100x - 250x"},
		{250, 500, "250x - 500x"},
		{500, 1000, "500x - 1000x"},
		{1000, 2500, "1000x - 2500x"},
		{2500, 5000, "2500x - 5000x"},
		{5000, 10000, "5000x - 10000x"},
		{10000, maxPayout + 1, fmt.Sprintf("10000x - %.0fx", maxPayout)},
	}

	result := make([]PayoutGapDetail, 0)

	for _, r := range ranges {
		if r.start > maxPayout {
			break
		}

		var weight uint64
		for _, o := range lut.Outcomes {
			payout := float64(o.Payout) / 100.0
			if payout >= r.start && payout < r.end {
				weight += o.Weight
			}
		}

		prob := 0.0
		if totalWeight > 0 {
			prob = float64(weight) / float64(totalWeight)
		}

		result = append(result, PayoutGapDetail{
			Range:       r.label,
			HasPayouts:  weight > 0,
			TotalWeight: weight,
			Probability: prob,
		})
	}

	// Sort by range start (already sorted by definition)
	return result
}
