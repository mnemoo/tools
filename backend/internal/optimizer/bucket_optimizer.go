package optimizer

import (
	"fmt"
	"math"
	"sort"

	"lutexplorer/internal/common"
	"lutexplorer/internal/lut"
	"stakergs"
)

// BucketConstraintType defines how a bucket's probability is specified
type BucketConstraintType string

const (
	// ConstraintFrequency specifies probability as "1 in N spins"
	ConstraintFrequency BucketConstraintType = "frequency"
	// ConstraintRTPPercent specifies probability via RTP contribution percentage
	ConstraintRTPPercent BucketConstraintType = "rtp_percent"
	// ConstraintAuto automatically uses remaining RTP after other buckets
	// Distributes weights inversely proportional to payout (higher payout = lower weight)
	ConstraintAuto BucketConstraintType = "auto"
)

// BucketConfig defines a payout range and its probability constraint
type BucketConfig struct {
	Name        string               `json:"name"`         // Human-readable name (e.g., "small_wins")
	MinPayout   float64              `json:"min_payout"`   // Minimum payout in range (inclusive)
	MaxPayout   float64              `json:"max_payout"`   // Maximum payout in range (exclusive, except for last bucket)
	Type        BucketConstraintType `json:"type"`         // "frequency", "rtp_percent", or "auto"
	Frequency   float64              `json:"frequency"`    // 1 in N spins (e.g., 20 = 1 in 20 spins)
	RTPPercent  float64              `json:"rtp_percent"`  // % of total RTP (e.g., 0.5 = 0.5% of RTP)
	AutoExponent float64             `json:"auto_exponent"` // For auto: weight ∝ 1/payout^exponent (default 1.0, higher = steeper)
}

// BucketOptimizerConfig contains full configuration for bucket-based optimization
type BucketOptimizerConfig struct {
	TargetRTP    float64        `json:"target_rtp"`    // Target RTP (e.g., 0.97)
	RTPTolerance float64        `json:"rtp_tolerance"` // Acceptable deviation (e.g., 0.001)
	Buckets      []BucketConfig `json:"buckets"`       // Payout range configurations
	MinWeight    uint64         `json:"min_weight"`    // Minimum weight for any outcome (default 1)
}

// DefaultBucketConfig returns a sensible default bucket configuration
func DefaultBucketConfig() *BucketOptimizerConfig {
	return &BucketOptimizerConfig{
		TargetRTP:    0.97,
		RTPTolerance: 0.001,
		MinWeight:    1,
		Buckets: []BucketConfig{
			{Name: "sub_1x", MinPayout: 0, MaxPayout: 1, Type: ConstraintFrequency, Frequency: 3},
			{Name: "small", MinPayout: 1, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 5},
			{Name: "medium", MinPayout: 5, MaxPayout: 20, Type: ConstraintFrequency, Frequency: 25},
			{Name: "large", MinPayout: 20, MaxPayout: 100, Type: ConstraintFrequency, Frequency: 100},
			{Name: "huge", MinPayout: 100, MaxPayout: 1000, Type: ConstraintRTPPercent, RTPPercent: 5},
			{Name: "jackpot", MinPayout: 1000, MaxPayout: 100000, Type: ConstraintRTPPercent, RTPPercent: 0.5},
		},
	}
}

// BucketOptimizer optimizes using user-defined payout buckets
type BucketOptimizer struct {
	config *BucketOptimizerConfig
}

// NewBucketOptimizer creates a new bucket optimizer
func NewBucketOptimizer(config *BucketOptimizerConfig) *BucketOptimizer {
	if config == nil {
		config = DefaultBucketConfig()
	}
	if config.MinWeight < 1 {
		config.MinWeight = 1
	}
	if config.RTPTolerance <= 0 {
		config.RTPTolerance = 0.001
	}
	return &BucketOptimizer{config: config}
}

// BucketResult contains details about a single bucket's optimization
type BucketResult struct {
	Name              string  `json:"name"`
	MinPayout         float64 `json:"min_payout"`
	MaxPayout         float64 `json:"max_payout"`
	OutcomeCount      int     `json:"outcome_count"`
	TargetProbability float64 `json:"target_probability"` // Target probability for bucket
	ActualProbability float64 `json:"actual_probability"` // Achieved probability
	TargetFrequency   float64 `json:"target_frequency"`   // 1 in N (derived)
	ActualFrequency   float64 `json:"actual_frequency"`   // 1 in N (achieved)
	RTPContribution   float64 `json:"rtp_contribution"`   // % of RTP this bucket contributes
	TotalWeight       uint64  `json:"total_weight"`       // Sum of weights in bucket
	AvgPayout         float64 `json:"avg_payout"`         // Average payout in bucket
}

// BucketOptimizerResult contains the full optimization result
type BucketOptimizerResult struct {
	OriginalRTP     float64               `json:"original_rtp"`
	FinalRTP        float64               `json:"final_rtp"`
	TargetRTP       float64               `json:"target_rtp"`
	Converged       bool                  `json:"converged"`
	NewWeights      []uint64              `json:"new_weights"`
	BucketResults   []BucketResult        `json:"bucket_results"`
	LossResult      *BucketResult         `json:"loss_result"`
	TotalWeight     uint64                `json:"total_weight"`
	Warnings        []string              `json:"warnings,omitempty"`
	OutcomeDetails  []OutcomeDetail       `json:"outcome_details,omitempty"`
}

// OutcomeDetail shows how each outcome was assigned
type OutcomeDetail struct {
	SimID      int     `json:"sim_id"`
	Payout     float64 `json:"payout"`
	OldWeight  uint64  `json:"old_weight"`
	NewWeight  uint64  `json:"new_weight"`
	BucketName string  `json:"bucket_name"`
	Probability float64 `json:"probability"`
}

// bucketAssignment holds outcomes assigned to a bucket during optimization
type bucketAssignment struct {
	config            BucketConfig
	outcomeIndices    []int
	payouts           []float64
	targetProb        float64   // Total probability for bucket (sum of outcomeProbs for auto)
	outcomeProbs      []float64 // Per-outcome probabilities (for auto buckets with varying probs)
	avgPayout         float64
	rtpContribution   float64
	isAuto            bool // True if this is an auto bucket
}

// OptimizeTable optimizes a lookup table using bucket constraints
func (o *BucketOptimizer) OptimizeTable(table *stakergs.LookupTable) (*BucketOptimizerResult, error) {
	n := len(table.Outcomes)
	if n == 0 {
		return nil, fmt.Errorf("empty table")
	}

	cost := table.Cost
	if cost <= 0 {
		cost = 1.0
	}

	// Extract payouts (normalized by cost)
	payouts := make([]float64, n)
	originalWeights := make([]uint64, n)
	for i, outcome := range table.Outcomes {
		payouts[i] = float64(outcome.Payout) / 100.0 / cost
		originalWeights[i] = outcome.Weight
	}

	originalRTP := calculateRTPFromWeights(originalWeights, payouts)

	// Assign outcomes to buckets
	assignments, lossIndices, warnings := o.assignOutcomesToBuckets(payouts)

	// Calculate target probabilities for each bucket
	probWarnings := o.calculateTargetProbabilities(assignments)
	warnings = append(warnings, probWarnings...)

	// Calculate weights
	newWeights, bucketResults, lossResult := o.calculateWeights(payouts, assignments, lossIndices)

	// Calculate final RTP
	finalRTP := calculateRTPFromWeights(newWeights, payouts)
	converged := math.Abs(finalRTP-o.config.TargetRTP) <= o.config.RTPTolerance

	// Fine-tune if not converged
	if !converged && len(lossIndices) > 0 {
		newWeights = o.fineTuneLossWeight(newWeights, payouts, lossIndices)
		finalRTP = calculateRTPFromWeights(newWeights, payouts)
		converged = math.Abs(finalRTP-o.config.TargetRTP) <= o.config.RTPTolerance

		// Recalculate loss result
		lossResult = o.calculateLossResult(newWeights, payouts, lossIndices)
	}

	// Add warning if final RTP is way off target
	if !converged {
		diff := (finalRTP - o.config.TargetRTP) * 100
		if diff > 10 {
			warnings = append(warnings, fmt.Sprintf(
				"Final RTP %.1f%% is %.0f%% above target. High-payout outcomes (with min weight=1) contribute too much RTP. Try removing high-payout buckets or using fewer frequency constraints.",
				finalRTP*100, diff))
		} else if diff < -10 {
			warnings = append(warnings, fmt.Sprintf(
				"Final RTP %.1f%% is %.0f%% below target. Not enough high-value outcomes to reach target RTP.",
				finalRTP*100, -diff))
		}
	}

	// Build outcome details
	outcomeDetails := o.buildOutcomeDetails(table, payouts, originalWeights, newWeights, assignments, lossIndices)

	return &BucketOptimizerResult{
		OriginalRTP:    originalRTP,
		FinalRTP:       finalRTP,
		TargetRTP:      o.config.TargetRTP,
		Converged:      converged,
		NewWeights:     newWeights,
		BucketResults:  bucketResults,
		LossResult:     lossResult,
		TotalWeight:    sumUint64(newWeights),
		Warnings:       warnings,
		OutcomeDetails: outcomeDetails,
	}, nil
}

// assignOutcomesToBuckets assigns each outcome to appropriate bucket
func (o *BucketOptimizer) assignOutcomesToBuckets(payouts []float64) ([]bucketAssignment, []int, []string) {
	var warnings []string
	var lossIndices []int

	// Create assignments for each bucket
	assignments := make([]bucketAssignment, len(o.config.Buckets))
	for i, bucket := range o.config.Buckets {
		assignments[i] = bucketAssignment{
			config:         bucket,
			outcomeIndices: []int{},
			payouts:        []float64{},
		}
	}

	// Assign each outcome
	for i, payout := range payouts {
		if payout <= 0 {
			lossIndices = append(lossIndices, i)
			continue
		}

		assigned := false
		for j := range assignments {
			bucket := &assignments[j]
			// Check if payout falls within this bucket's range
			// Last bucket includes max (>=), others exclude it (<)
			inRange := payout >= bucket.config.MinPayout
			if j < len(assignments)-1 {
				inRange = inRange && payout < bucket.config.MaxPayout
			} else {
				inRange = inRange && payout <= bucket.config.MaxPayout
			}

			if inRange {
				bucket.outcomeIndices = append(bucket.outcomeIndices, i)
				bucket.payouts = append(bucket.payouts, payout)
				assigned = true
				break
			}
		}

		if !assigned {
			// Outcome doesn't fit any bucket - silently assign to closest
			// Find closest bucket
			closestIdx := 0
			closestDist := math.MaxFloat64
			for j, bucket := range assignments {
				dist := math.Min(
					math.Abs(payout-bucket.config.MinPayout),
					math.Abs(payout-bucket.config.MaxPayout),
				)
				if dist < closestDist {
					closestDist = dist
					closestIdx = j
				}
			}
			assignments[closestIdx].outcomeIndices = append(assignments[closestIdx].outcomeIndices, i)
			assignments[closestIdx].payouts = append(assignments[closestIdx].payouts, payout)
		}
	}

	// Calculate average payout for each bucket
	for i := range assignments {
		if len(assignments[i].payouts) > 0 {
			sum := 0.0
			for _, p := range assignments[i].payouts {
				sum += p
			}
			assignments[i].avgPayout = sum / float64(len(assignments[i].payouts))
		}
	}

	return assignments, lossIndices, warnings
}

// calculateTargetProbabilities calculates target probability for each bucket
// For auto buckets, it first calculates non-auto buckets, then distributes remaining RTP
// Returns warnings if constraints are impossible to satisfy
func (o *BucketOptimizer) calculateTargetProbabilities(assignments []bucketAssignment) []string {
	var warnings []string
	// First pass: calculate probabilities for frequency and rtp_percent buckets
	var usedRTP float64

	for i := range assignments {
		bucket := &assignments[i]
		if len(bucket.outcomeIndices) == 0 {
			continue
		}

		switch bucket.config.Type {
		case ConstraintFrequency:
			// Frequency: 1 in N spins = probability of 1/N
			if bucket.config.Frequency > 0 {
				bucket.targetProb = 1.0 / bucket.config.Frequency
			}
			// Calculate implied RTP contribution
			bucket.rtpContribution = bucket.targetProb * bucket.avgPayout
			usedRTP += bucket.rtpContribution

		case ConstraintRTPPercent:
			// RTP%: X% of target RTP
			// RTP contribution = rtpPercent/100 * targetRTP
			// probability = contribution / avgPayout
			if bucket.avgPayout > 0 && bucket.config.RTPPercent > 0 {
				bucket.rtpContribution = (bucket.config.RTPPercent / 100.0) * o.config.TargetRTP
				bucket.targetProb = bucket.rtpContribution / bucket.avgPayout
				usedRTP += bucket.rtpContribution
			}

		case ConstraintAuto:
			bucket.isAuto = true
			// Will be calculated in second pass
		}
	}

	// Second pass: distribute remaining RTP to auto buckets
	remainingRTP := o.config.TargetRTP - usedRTP
	if remainingRTP < 0 {
		remainingRTP = 0
	}

	// Track if frequency buckets already exceed target RTP
	if usedRTP > o.config.TargetRTP {
		warnings = append(warnings, fmt.Sprintf(
			"Frequency/RTP%% constraints already use %.1f%% RTP (target: %.1f%%). Cannot reach target RTP. Reduce frequencies or use AUTO type.",
			usedRTP*100, o.config.TargetRTP*100))
	}

	// Collect all auto bucket outcomes
	var autoBucketIndices []int
	var totalAutoOutcomes int
	for i := range assignments {
		if assignments[i].isAuto && len(assignments[i].outcomeIndices) > 0 {
			autoBucketIndices = append(autoBucketIndices, i)
			totalAutoOutcomes += len(assignments[i].outcomeIndices)
		}
	}

	if len(autoBucketIndices) > 0 && remainingRTP > 0 {
		// For auto buckets, use inverse-proportional distribution:
		// prob_i = remainingRTP * (1/payout_i^exp) / Σ(payout_j^(1-exp))
		//
		// This ensures:
		// 1. Higher payouts get lower weights
		// 2. Total RTP contribution equals remainingRTP
		// 3. Each outcome contributes equally to RTP (with exp=1)

		// Calculate sum of payout^(1-exp) across all auto outcomes
		var sumPayout1MinusExp float64
		for _, bucketIdx := range autoBucketIndices {
			bucket := &assignments[bucketIdx]
			exp := bucket.config.AutoExponent
			if exp <= 0 {
				exp = 1.0 // Default exponent
			}
			for _, p := range bucket.payouts {
				if p > 0 {
					sumPayout1MinusExp += math.Pow(p, 1-exp)
				}
			}
		}

		// Distribute to each auto bucket
		for _, bucketIdx := range autoBucketIndices {
			bucket := &assignments[bucketIdx]
			exp := bucket.config.AutoExponent
			if exp <= 0 {
				exp = 1.0
			}

			bucket.outcomeProbs = make([]float64, len(bucket.payouts))
			var bucketTotalProb float64
			var bucketRTP float64

			for j, p := range bucket.payouts {
				if p > 0 && sumPayout1MinusExp > 0 {
					// prob_i = remainingRTP * (1/p^exp) / Σ(p^(1-exp))
					prob := remainingRTP * math.Pow(p, -exp) / sumPayout1MinusExp
					bucket.outcomeProbs[j] = prob
					bucketTotalProb += prob
					bucketRTP += prob * p
				}
			}

			bucket.targetProb = bucketTotalProb
			bucket.rtpContribution = bucketRTP
		}
	}

	return warnings
}

// calculateWeights converts probabilities to weights
func (o *BucketOptimizer) calculateWeights(payouts []float64, assignments []bucketAssignment, lossIndices []int) ([]uint64, []BucketResult, *BucketResult) {
	n := len(payouts)
	weights := make([]uint64, n)

	// Use large base for precision
	baseWeight := common.BaseWeight

	// Calculate total win probability and RTP contribution
	var totalWinProb float64
	var totalWinRTP float64

	bucketResults := make([]BucketResult, 0, len(assignments))

	for _, bucket := range assignments {
		if len(bucket.outcomeIndices) == 0 {
			continue
		}

		var actualTotalWeight uint64

		if bucket.isAuto && len(bucket.outcomeProbs) == len(bucket.outcomeIndices) {
			// Auto bucket: use per-outcome probabilities
			for j, idx := range bucket.outcomeIndices {
				prob := bucket.outcomeProbs[j]
				w := uint64(prob * float64(baseWeight))
				if w < o.config.MinWeight {
					w = o.config.MinWeight
				}
				weights[idx] = w
				actualTotalWeight += w
			}
		} else {
			// Non-auto bucket: distribute evenly
			bucketTotalWeight := uint64(bucket.targetProb * float64(baseWeight))
			weightPerOutcome := bucketTotalWeight / uint64(len(bucket.outcomeIndices))

			if weightPerOutcome < o.config.MinWeight {
				weightPerOutcome = o.config.MinWeight
			}

			for _, idx := range bucket.outcomeIndices {
				weights[idx] = weightPerOutcome
				actualTotalWeight += weightPerOutcome
			}
		}

		totalWinProb += bucket.targetProb
		totalWinRTP += bucket.rtpContribution

		// Record bucket result
		targetFreq := 0.0
		if bucket.targetProb > 0 {
			targetFreq = 1.0 / bucket.targetProb
		}

		bucketResults = append(bucketResults, BucketResult{
			Name:              bucket.config.Name,
			MinPayout:         bucket.config.MinPayout,
			MaxPayout:         bucket.config.MaxPayout,
			OutcomeCount:      len(bucket.outcomeIndices),
			TargetProbability: bucket.targetProb,
			TargetFrequency:   targetFreq,
			RTPContribution:   bucket.rtpContribution * 100, // As absolute % RTP
			TotalWeight:       actualTotalWeight,
			AvgPayout:         bucket.avgPayout,
		})
	}

	// Calculate loss weight
	// RTP = totalWinRTP + 0 (loss contributes 0)
	// We need: totalWinRTP = targetRTP
	// Loss probability = 1 - totalWinProb
	//
	// Actually, we need to adjust. Let's calculate:
	// Current win RTP = totalWinRTP
	// If totalWinRTP > targetRTP, we need more loss
	// If totalWinRTP < targetRTP, we need less loss (or can't achieve target)

	// The relationship is:
	// RTP = Σ(p_i * payout_i) where Σp_i = 1
	// Let p_loss = 1 - totalWinProb
	// RTP = totalWinRTP (since loss * 0 = 0)
	//
	// But we distributed based on target probs, not actual probs.
	// The actual prob depends on total weight.
	//
	// Let's work backwards:
	// totalWinWeight = sum of bucket weights
	// We want: totalWinRTP = targetRTP
	// actualRTP = Σ(weight_i * payout_i) / totalWeight
	//
	// Set loss weight such that:
	// Σ(winWeight * payout) / (winWeight + lossWeight) = targetRTP
	// weightedPayoutSum / (totalWinWeight + lossWeight) = targetRTP
	// lossWeight = weightedPayoutSum / targetRTP - totalWinWeight

	var weightedPayoutSum float64
	var totalWinWeight uint64
	for i, w := range weights {
		if payouts[i] > 0 {
			weightedPayoutSum += float64(w) * payouts[i]
			totalWinWeight += w
		}
	}

	// Required loss weight
	requiredLossWeight := weightedPayoutSum/o.config.TargetRTP - float64(totalWinWeight)
	if requiredLossWeight < float64(o.config.MinWeight) {
		requiredLossWeight = float64(o.config.MinWeight)
	}

	// Distribute loss weight among loss outcomes
	var lossResult *BucketResult
	if len(lossIndices) > 0 {
		lossWeightPerOutcome := uint64(math.Round(requiredLossWeight / float64(len(lossIndices))))
		if lossWeightPerOutcome < o.config.MinWeight {
			lossWeightPerOutcome = o.config.MinWeight
		}

		var totalLossWeight uint64
		for _, idx := range lossIndices {
			weights[idx] = lossWeightPerOutcome
			totalLossWeight += lossWeightPerOutcome
		}

		totalWeight := totalWinWeight + totalLossWeight
		lossProb := float64(totalLossWeight) / float64(totalWeight)

		lossResult = &BucketResult{
			Name:              "loss",
			MinPayout:         0,
			MaxPayout:         0,
			OutcomeCount:      len(lossIndices),
			TargetProbability: 1 - totalWinProb,
			ActualProbability: lossProb,
			TargetFrequency:   1.0 / (1 - totalWinProb),
			ActualFrequency:   1.0 / lossProb,
			RTPContribution:   0,
			TotalWeight:       totalLossWeight,
			AvgPayout:         0,
		}
	}

	// Update bucket results with actual probabilities and RTP contributions
	totalWeight := sumUint64(weights)
	for i := range bucketResults {
		bucketResults[i].ActualProbability = float64(bucketResults[i].TotalWeight) / float64(totalWeight)
		bucketResults[i].ActualFrequency = 1.0 / bucketResults[i].ActualProbability
		// Recalculate RTP contribution based on actual probability
		bucketResults[i].RTPContribution = bucketResults[i].ActualProbability * bucketResults[i].AvgPayout * 100
	}

	return weights, bucketResults, lossResult
}

// fineTuneLossWeight adjusts loss weight to hit target RTP precisely
func (o *BucketOptimizer) fineTuneLossWeight(weights []uint64, payouts []float64, lossIndices []int) []uint64 {
	result := make([]uint64, len(weights))
	copy(result, weights)

	// Calculate weighted payout sum for wins
	var weightedPayoutSum float64
	var totalWinWeight uint64
	for i, p := range payouts {
		if p > 0 {
			weightedPayoutSum += float64(result[i]) * p
			totalWinWeight += result[i]
		}
	}

	// Required loss weight for target RTP
	requiredLossWeight := weightedPayoutSum/o.config.TargetRTP - float64(totalWinWeight)
	if requiredLossWeight < float64(len(lossIndices)) {
		requiredLossWeight = float64(len(lossIndices))
	}

	// Distribute among loss outcomes
	lossWeightPerOutcome := uint64(math.Round(requiredLossWeight / float64(len(lossIndices))))
	if lossWeightPerOutcome < o.config.MinWeight {
		lossWeightPerOutcome = o.config.MinWeight
	}

	for _, idx := range lossIndices {
		result[idx] = lossWeightPerOutcome
	}

	return result
}

// calculateLossResult recalculates loss bucket result after fine-tuning
func (o *BucketOptimizer) calculateLossResult(weights []uint64, payouts []float64, lossIndices []int) *BucketResult {
	var totalLossWeight uint64
	for _, idx := range lossIndices {
		totalLossWeight += weights[idx]
	}

	totalWeight := sumUint64(weights)
	lossProb := float64(totalLossWeight) / float64(totalWeight)

	return &BucketResult{
		Name:              "loss",
		MinPayout:         0,
		MaxPayout:         0,
		OutcomeCount:      len(lossIndices),
		ActualProbability: lossProb,
		ActualFrequency:   1.0 / lossProb,
		RTPContribution:   0,
		TotalWeight:       totalLossWeight,
		AvgPayout:         0,
	}
}

// buildOutcomeDetails creates detailed info for each outcome
func (o *BucketOptimizer) buildOutcomeDetails(table *stakergs.LookupTable, payouts []float64, oldWeights, newWeights []uint64, assignments []bucketAssignment, lossIndices []int) []OutcomeDetail {
	totalWeight := sumUint64(newWeights)
	details := make([]OutcomeDetail, len(payouts))

	// Create index to bucket name mapping
	bucketNames := make(map[int]string)
	for _, bucket := range assignments {
		for _, idx := range bucket.outcomeIndices {
			bucketNames[idx] = bucket.config.Name
		}
	}
	for _, idx := range lossIndices {
		bucketNames[idx] = "loss"
	}

	for i := range payouts {
		details[i] = OutcomeDetail{
			SimID:       table.Outcomes[i].SimID,
			Payout:      payouts[i] * table.Cost, // De-normalize
			OldWeight:   oldWeights[i],
			NewWeight:   newWeights[i],
			BucketName:  bucketNames[i],
			Probability: float64(newWeights[i]) / float64(totalWeight),
		}
	}

	return details
}

// OptimizeFromLoader loads a mode and optimizes it
func (o *BucketOptimizer) OptimizeFromLoader(loader *lut.Loader, mode string) (*BucketOptimizerResult, error) {
	table, err := loader.GetMode(mode)
	if err != nil {
		return nil, fmt.Errorf("failed to load mode %s: %w", mode, err)
	}
	return o.OptimizeTable(table)
}

// ValidateBuckets checks if bucket configuration is valid
func ValidateBuckets(buckets []BucketConfig) error {
	if len(buckets) == 0 {
		return fmt.Errorf("at least one bucket required")
	}

	// Sort by MinPayout
	sorted := make([]BucketConfig, len(buckets))
	copy(sorted, buckets)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].MinPayout < sorted[j].MinPayout
	})

	// Check for gaps and overlaps
	for i := 0; i < len(sorted)-1; i++ {
		if sorted[i].MaxPayout < sorted[i+1].MinPayout {
			return fmt.Errorf("gap between buckets: %.2f-%.2f and %.2f-%.2f",
				sorted[i].MinPayout, sorted[i].MaxPayout,
				sorted[i+1].MinPayout, sorted[i+1].MaxPayout)
		}
		if sorted[i].MaxPayout > sorted[i+1].MinPayout {
			return fmt.Errorf("overlap between buckets: %.2f-%.2f and %.2f-%.2f",
				sorted[i].MinPayout, sorted[i].MaxPayout,
				sorted[i+1].MinPayout, sorted[i+1].MaxPayout)
		}
	}

	// Validate each bucket
	autoCount := 0
	for _, bucket := range buckets {
		if bucket.MinPayout < 0 {
			return fmt.Errorf("bucket %s: min_payout cannot be negative", bucket.Name)
		}
		if bucket.MaxPayout <= bucket.MinPayout {
			return fmt.Errorf("bucket %s: max_payout must be > min_payout", bucket.Name)
		}

		switch bucket.Type {
		case ConstraintFrequency:
			if bucket.Frequency <= 0 {
				return fmt.Errorf("bucket %s: frequency must be > 0", bucket.Name)
			}
		case ConstraintRTPPercent:
			if bucket.RTPPercent <= 0 || bucket.RTPPercent > 100 {
				return fmt.Errorf("bucket %s: rtp_percent must be between 0 and 100", bucket.Name)
			}
		case ConstraintAuto:
			autoCount++
			// AutoExponent is optional, defaults to 1.0 in calculateTargetProbabilities
			if bucket.AutoExponent < 0 {
				return fmt.Errorf("bucket %s: auto_exponent cannot be negative", bucket.Name)
			}
		default:
			return fmt.Errorf("bucket %s: unknown constraint type %s", bucket.Name, bucket.Type)
		}
	}

	// Warn if multiple auto buckets (allowed but unusual)
	if autoCount > 1 {
		// This is fine - multiple auto buckets will share remaining RTP
	}

	return nil
}

// SuggestBuckets analyzes a table and suggests bucket configuration
// For high-cost modes (bonus), generates buckets adapted to normalized payouts
func SuggestBuckets(table *stakergs.LookupTable, targetRTP float64) []BucketConfig {
	cost := table.Cost
	if cost <= 0 {
		cost = 1.0
	}

	// Find payout ranges (normalized by cost)
	var maxPayout, minPayout float64
	minPayout = math.MaxFloat64
	for _, outcome := range table.Outcomes {
		payout := float64(outcome.Payout) / 100.0 / cost
		if payout > maxPayout {
			maxPayout = payout
		}
		if payout > 0 && payout < minPayout {
			minPayout = payout
		}
	}

	if maxPayout <= 0 {
		return []BucketConfig{}
	}

	// For bonus modes (cost > 1), all normalized payouts are typically < 2x
	// Generate buckets based on actual payout distribution
	if cost > 1.5 {
		return suggestBonusBuckets(minPayout, maxPayout, targetRTP)
	}

	// Standard mode buckets
	return suggestStandardBuckets(maxPayout)
}

// suggestBonusBuckets generates buckets for high-cost modes (bonus)
// where normalized payouts are typically clustered around targetRTP
func suggestBonusBuckets(minPayout, maxPayout, targetRTP float64) []BucketConfig {
	buckets := []BucketConfig{}

	// For bonus modes, payouts are typically 0.5x - 1.5x normalized
	// The distribution is usually tight around the target RTP

	// Split into 3-4 buckets based on actual range
	payoutRange := maxPayout - minPayout
	if payoutRange <= 0 {
		payoutRange = 1.0
	}

	// Low payouts: below target RTP
	// Always start from 0 to catch all positive payouts
	lowThreshold := targetRTP * 0.8
	buckets = append(buckets, BucketConfig{
		Name:         "below_avg",
		MinPayout:    0, // Start from 0 to catch all payouts
		MaxPayout:    lowThreshold,
		Type:         ConstraintAuto,
		AutoExponent: 1.0,
	})

	// Around target RTP (most common)
	midLow := lowThreshold
	midHigh := targetRTP * 1.2
	if midHigh > maxPayout {
		midHigh = maxPayout * 0.9
	}
	buckets = append(buckets, BucketConfig{
		Name:         "around_avg",
		MinPayout:    midLow,
		MaxPayout:    midHigh,
		Type:         ConstraintAuto,
		AutoExponent: 1.0,
	})

	// Above average (good bonus outcomes)
	if maxPayout > midHigh {
		highThreshold := targetRTP * 1.5
		if highThreshold < midHigh {
			highThreshold = midHigh * 1.2
		}
		if highThreshold > maxPayout {
			highThreshold = maxPayout + 0.01
		}

		buckets = append(buckets, BucketConfig{
			Name:         "above_avg",
			MinPayout:    midHigh,
			MaxPayout:    highThreshold,
			Type:         ConstraintRTPPercent,
			RTPPercent:   15, // 15% of RTP for good outcomes
		})

		// Jackpot tier (if exists)
		if maxPayout > highThreshold {
			buckets = append(buckets, BucketConfig{
				Name:       "jackpot",
				MinPayout:  highThreshold,
				MaxPayout:  maxPayout + 0.01,
				Type:       ConstraintRTPPercent,
				RTPPercent: 5, // 5% of RTP for jackpots
			})
		}
	}

	return buckets
}

// suggestStandardBuckets generates buckets for normal modes (cost = 1)
func suggestStandardBuckets(maxPayout float64) []BucketConfig {
	buckets := []BucketConfig{}

	// Sub-1x wins (if exist)
	buckets = append(buckets, BucketConfig{
		Name:      "sub_1x",
		MinPayout: 0.01,
		MaxPayout: 1,
		Type:      ConstraintFrequency,
		Frequency: 3, // 1 in 3
	})

	// Small wins: 1x-5x
	if maxPayout >= 1 {
		buckets = append(buckets, BucketConfig{
			Name:      "small",
			MinPayout: 1,
			MaxPayout: 5,
			Type:      ConstraintFrequency,
			Frequency: 6, // 1 in 6
		})
	}

	// Medium wins: 5x-20x
	if maxPayout >= 5 {
		buckets = append(buckets, BucketConfig{
			Name:      "medium",
			MinPayout: 5,
			MaxPayout: 20,
			Type:      ConstraintFrequency,
			Frequency: 25, // 1 in 25
		})
	}

	// Large wins: 20x-100x
	if maxPayout >= 20 {
		buckets = append(buckets, BucketConfig{
			Name:      "large",
			MinPayout: 20,
			MaxPayout: 100,
			Type:      ConstraintFrequency,
			Frequency: 100, // 1 in 100
		})
	}

	// Huge wins: 100x-1000x (RTP-based)
	if maxPayout >= 100 {
		buckets = append(buckets, BucketConfig{
			Name:       "huge",
			MinPayout:  100,
			MaxPayout:  1000,
			Type:       ConstraintRTPPercent,
			RTPPercent: 5, // 5% of RTP
		})
	}

	// Jackpot: 1000x+ (RTP-based)
	if maxPayout >= 1000 {
		buckets = append(buckets, BucketConfig{
			Name:       "jackpot",
			MinPayout:  1000,
			MaxPayout:  maxPayout + 1,
			Type:       ConstraintRTPPercent,
			RTPPercent: 0.5, // 0.5% of RTP
		})
	}

	return buckets
}
