// Package lut provides types and utilities for LUT analysis.
package lut

import (
	"fmt"
	"math"
	"sort"

	"stakergs"
)

// round2 rounds to 2 decimal places (for payouts/multipliers - min 0.01x)
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// round4 rounds to 4 decimal places (for rates/ratios)
func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// Statistics contains comprehensive LUT analysis results.
type Statistics struct {
	Mode           string             `json:"mode"`
	TotalOutcomes  int                `json:"total_outcomes"`
	TotalWeight    uint64             `json:"total_weight"`
	RTP            float64            `json:"rtp"`
	HitRate        float64            `json:"hit_rate"`
	MaxPayout      float64            `json:"max_payout"`
	MinPayout      float64            `json:"min_payout"`
	MeanPayout     float64            `json:"mean_payout"`
	MedianPayout   float64            `json:"median_payout"`
	Variance       float64            `json:"variance"`
	StdDev         float64            `json:"std_dev"`
	Volatility     float64            `json:"volatility"`
	MeanMedian     float64            `json:"mean_median_ratio"`
	PayoutBuckets  []PayoutBucket     `json:"payout_buckets"`
	Distribution   []DistributionItem `json:"distribution"`
	TopPayouts     []PayoutInfo       `json:"top_payouts"`
	ZeroPayoutRate float64            `json:"zero_payout_rate"`
}

// PayoutBucket represents a range of payouts for histogram visualization.
type PayoutBucket struct {
	RangeStart  float64 `json:"range_start"`
	RangeEnd    float64 `json:"range_end"`
	Count       int     `json:"count"`
	Weight      uint64  `json:"weight"`
	Probability float64 `json:"probability"`
}

// DistributionItem represents a single payout value with its statistics.
type DistributionItem struct {
	Payout   float64 `json:"payout"`
	Weight   uint64  `json:"weight"`
	Odds     string  `json:"odds"`      // formatted "1 in 1.5M"
	Count    int     `json:"count"`     // number of sim_ids with this payout
	SimIDs   []int   `json:"sim_ids"`   // first few sim_ids for quick lookup (max 10)
}

// PayoutInfo represents information about a specific payout outcome.
type PayoutInfo struct {
	SimID  int     `json:"sim_id"`
	Payout float64 `json:"payout"`
	Weight uint64  `json:"weight"`
	Odds   string  `json:"odds"`  // formatted "1 in 1.5M"
	Count  int     `json:"count"` // number of outcomes with this payout
}

// Analyzer provides analysis utilities for LookupTables.
type Analyzer struct{}

// NewAnalyzer creates a new Analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// MinNonZeroPayout returns the minimum non-zero payout multiplier.
func (a *Analyzer) MinNonZeroPayout(lut *stakergs.LookupTable) float64 {
	var min uint = math.MaxUint32
	for _, o := range lut.Outcomes {
		if o.Payout > 0 && o.Payout < min {
			min = o.Payout
		}
	}
	if min == math.MaxUint32 {
		return 0
	}
	return float64(min) / 100.0
}

// Analyze performs comprehensive analysis on the LUT.
func (a *Analyzer) Analyze(lut *stakergs.LookupTable) *Statistics {
	totalWeight := lut.TotalWeight()
	if totalWeight == 0 || len(lut.Outcomes) == 0 {
		return &Statistics{Mode: lut.Mode}
	}

	stats := &Statistics{
		Mode:          lut.Mode,
		TotalOutcomes: len(lut.Outcomes),
		TotalWeight:   totalWeight,
		RTP:           round4(lut.RTP()),
		HitRate:       round4(lut.HitRate()),
		MaxPayout:     round2(float64(lut.MaxPayout()) / 100.0),
		MinPayout:     round2(a.MinNonZeroPayout(lut)),
	}

	// Calculate mean payout (weighted)
	var weightedSum float64
	for _, o := range lut.Outcomes {
		payout := float64(o.Payout) / 100.0
		prob := float64(o.Weight) / float64(totalWeight)
		weightedSum += payout * prob
	}
	stats.MeanPayout = round4(weightedSum)

	// Calculate variance and std dev
	var variance float64
	for _, o := range lut.Outcomes {
		payout := float64(o.Payout) / 100.0
		prob := float64(o.Weight) / float64(totalWeight)
		diff := payout - weightedSum // use unrounded for calculation
		variance += diff * diff * prob
	}
	stats.Variance = round4(variance)
	stats.StdDev = round4(math.Sqrt(variance))
	if weightedSum > 0 {
		stats.Volatility = round4(math.Sqrt(variance) / weightedSum)
	}

	// Calculate median payout (weighted)
	stats.MedianPayout = round2(a.calculateWeightedMedian(lut, totalWeight))

	// Mean/Median ratio (volatility indicator)
	if stats.MedianPayout > 0 {
		stats.MeanMedian = round4(weightedSum / a.calculateWeightedMedian(lut, totalWeight))
	}

	// Zero payout rate
	var zeroWeight uint64
	for _, o := range lut.Outcomes {
		if o.Payout == 0 {
			zeroWeight += o.Weight
		}
	}
	stats.ZeroPayoutRate = round4(float64(zeroWeight) / float64(totalWeight))

	// Distribution (sorted by payout)
	stats.Distribution = a.BuildDistribution(lut, totalWeight)

	// Payout buckets for histogram
	stats.PayoutBuckets = a.buildPayoutBuckets(lut, totalWeight)

	// Top payouts
	stats.TopPayouts = a.getTopPayouts(lut, totalWeight, 10)

	return stats
}

func (a *Analyzer) calculateWeightedMedian(lut *stakergs.LookupTable, totalWeight uint64) float64 {
	// Sort outcomes by payout
	sorted := make([]stakergs.Outcome, len(lut.Outcomes))
	copy(sorted, lut.Outcomes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Payout < sorted[j].Payout
	})

	// Find median
	halfWeight := float64(totalWeight) / 2.0
	var cumWeight uint64
	for _, o := range sorted {
		cumWeight += o.Weight
		if float64(cumWeight) >= halfWeight {
			return float64(o.Payout) / 100.0
		}
	}
	return 0
}

// payoutData holds aggregated data for a payout value
type payoutData struct {
	weight uint64
	simIDs []int
}

// BuildDistribution builds the payout distribution from a LUT.
func (a *Analyzer) BuildDistribution(lut *stakergs.LookupTable, totalWeight uint64) []DistributionItem {
	// Group by payout value
	payoutMap := make(map[uint]*payoutData)
	for _, o := range lut.Outcomes {
		if payoutMap[o.Payout] == nil {
			payoutMap[o.Payout] = &payoutData{}
		}
		payoutMap[o.Payout].weight += o.Weight
		payoutMap[o.Payout].simIDs = append(payoutMap[o.Payout].simIDs, o.SimID)
	}

	// Convert to sorted slice
	items := make([]DistributionItem, 0, len(payoutMap))
	for payout, data := range payoutMap {
		odds := float64(totalWeight) / float64(data.weight)

		// Keep only first 10 sim_ids for the response
		simIDs := data.simIDs
		if len(simIDs) > 10 {
			simIDs = simIDs[:10]
		}

		items = append(items, DistributionItem{
			Payout: round2(float64(payout) / 100.0),
			Weight: data.weight,
			Odds:   formatOdds(odds),
			Count:  len(data.simIDs),
			SimIDs: simIDs,
		})
	}

	// Sort by payout descending (highest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Payout > items[j].Payout
	})

	return items
}

// formatOdds formats odds as "1 in X" with appropriate suffix
func formatOdds(odds float64) string {
	if odds >= 1_000_000_000 {
		return fmt.Sprintf("1 in %.2fB", odds/1_000_000_000)
	}
	if odds >= 1_000_000 {
		return fmt.Sprintf("1 in %.2fM", odds/1_000_000)
	}
	if odds >= 1_000 {
		return fmt.Sprintf("1 in %.2fK", odds/1_000)
	}
	if odds >= 10 {
		return fmt.Sprintf("1 in %.0f", odds)
	}
	return fmt.Sprintf("1 in %.2f", odds)
}

// generateBucketBoundaries creates logarithmic bucket boundaries using 1-2-5 pattern
// up to the specified max value.
func generateBucketBoundaries(maxPayout float64) []float64 {
	// Start with special boundaries for low payouts
	boundaries := []float64{0, 0.01, 1}

	// 1-2-5 pattern multipliers within each decade
	multipliers := []float64{1, 2, 5}

	// Generate boundaries from 1 up to maxPayout
	decade := 1.0
	for {
		for _, m := range multipliers {
			boundary := decade * m
			if boundary > 1 && boundary <= maxPayout*1.1 { // slight margin
				boundaries = append(boundaries, boundary)
			}
			if boundary > maxPayout*1.1 {
				return boundaries
			}
		}
		decade *= 10
		if decade > maxPayout*100 { // safety limit
			break
		}
	}

	return boundaries
}

func (a *Analyzer) buildPayoutBuckets(lut *stakergs.LookupTable, totalWeight uint64) []PayoutBucket {
	maxPayout := float64(lut.MaxPayout()) / 100.0
	if maxPayout == 0 {
		return nil
	}

	// Generate dynamic bucket boundaries
	boundaries := generateBucketBoundaries(maxPayout)

	// Create buckets from boundaries
	buckets := make([]PayoutBucket, 0, len(boundaries))

	// First bucket: exact zero (losses)
	buckets = append(buckets, PayoutBucket{RangeStart: 0, RangeEnd: 0})

	// Remaining buckets from boundaries
	for i := 1; i < len(boundaries); i++ {
		buckets = append(buckets, PayoutBucket{
			RangeStart: boundaries[i-1],
			RangeEnd:   boundaries[i],
		})
	}

	// Last bucket extends to max payout + epsilon
	if len(buckets) > 0 {
		lastBoundary := boundaries[len(boundaries)-1]
		if lastBoundary < maxPayout {
			buckets = append(buckets, PayoutBucket{
				RangeStart: lastBoundary,
				RangeEnd:   maxPayout + 1,
			})
		}
	}

	// Populate buckets with outcomes
	for _, o := range lut.Outcomes {
		payout := float64(o.Payout) / 100.0
		for i := range buckets {
			inBucket := false
			if buckets[i].RangeStart == 0 && buckets[i].RangeEnd == 0 {
				// Zero bucket: exact match
				inBucket = payout == 0
			} else {
				// Regular bucket: [start, end)
				inBucket = payout >= buckets[i].RangeStart && payout < buckets[i].RangeEnd
			}
			if inBucket {
				buckets[i].Count++
				buckets[i].Weight += o.Weight
				break
			}
		}
	}

	// Calculate probabilities and filter empty buckets
	result := make([]PayoutBucket, 0)
	for _, b := range buckets {
		if b.Weight > 0 {
			b.Probability = float64(b.Weight) / float64(totalWeight)
			result = append(result, b)
		}
	}

	return result
}

func (a *Analyzer) getTopPayouts(lut *stakergs.LookupTable, totalWeight uint64, limit int) []PayoutInfo {
	// Group outcomes by payout value
	type payoutGroup struct {
		payout      uint
		totalWeight uint64
		count       int
		firstSimID  int
		minWeight   uint64 // for finding rarest outcome
	}

	groups := make(map[uint]*payoutGroup)
	for _, o := range lut.Outcomes {
		if g, ok := groups[o.Payout]; ok {
			g.totalWeight += o.Weight
			g.count++
			// Track the rarest (lowest weight) outcome for this payout
			if o.Weight < g.minWeight {
				g.minWeight = o.Weight
				g.firstSimID = o.SimID
			}
		} else {
			groups[o.Payout] = &payoutGroup{
				payout:      o.Payout,
				totalWeight: o.Weight,
				count:       1,
				firstSimID:  o.SimID,
				minWeight:   o.Weight,
			}
		}
	}

	// Convert to slice and sort by payout descending
	groupList := make([]*payoutGroup, 0, len(groups))
	for _, g := range groups {
		groupList = append(groupList, g)
	}
	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].payout > groupList[j].payout
	})

	// Take top N unique payouts
	if len(groupList) > limit {
		groupList = groupList[:limit]
	}

	result := make([]PayoutInfo, len(groupList))
	for i, g := range groupList {
		odds := float64(totalWeight) / float64(g.totalWeight)
		result[i] = PayoutInfo{
			SimID:  g.firstSimID,
			Payout: round2(float64(g.payout) / 100.0),
			Weight: g.totalWeight,
			Odds:   formatOdds(odds),
			Count:  g.count,
		}
	}

	return result
}
