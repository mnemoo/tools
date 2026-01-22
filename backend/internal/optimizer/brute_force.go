package optimizer

import (
	"math"
	"time"

	"lutexplorer/internal/common"
	"stakergs"
)

// BruteForceOptimizer performs iterative optimization to hit target RTP precisely
type BruteForceOptimizer struct {
	config       *BucketOptimizerConfig
	progressChan chan<- BruteForceProgress
	stopChan     <-chan struct{} // Channel to signal stop
}

// NewBruteForceOptimizer creates a new brute force optimizer
func NewBruteForceOptimizer(config *BucketOptimizerConfig, progressChan chan<- BruteForceProgress) *BruteForceOptimizer {
	return NewBruteForceOptimizerWithStop(config, progressChan, nil)
}

// NewBruteForceOptimizerWithStop creates a new brute force optimizer with stop channel
func NewBruteForceOptimizerWithStop(config *BucketOptimizerConfig, progressChan chan<- BruteForceProgress, stopChan <-chan struct{}) *BruteForceOptimizer {
	if config == nil {
		config = DefaultBucketConfig()
	}
	if config.MinWeight < 1 {
		config.MinWeight = 1
	}
	if config.RTPTolerance <= 0 {
		config.RTPTolerance = 0.0001 // 0.01% default for brute force
	}
	if config.MaxIterations <= 0 {
		// Unlimited mode: run until converged or stopped (max 1M iterations as safety)
		config.MaxIterations = 1000000
	}
	return &BruteForceOptimizer{
		config:       config,
		progressChan: progressChan,
		stopChan:     stopChan,
	}
}

// getDefaultIterations returns default iteration count based on mode
func getDefaultIterations(mode OptimizationMode) int {
	switch mode {
	case ModeFast:
		return 100
	case ModePrecise:
		return 10000
	default: // ModeBalanced
		return 1000
	}
}

// bestSearchResult tracks the best result found during optimization
type bestSearchResult struct {
	weights []uint64
	rtp     float64
	error   float64
}

// copyWeights creates a copy of weights slice
func copyWeights(weights []uint64) []uint64 {
	result := make([]uint64, len(weights))
	copy(result, weights)
	return result
}

// isStopped checks if stop was requested (non-blocking)
func (o *BruteForceOptimizer) isStopped() bool {
	if o.stopChan == nil {
		return false
	}
	select {
	case <-o.stopChan:
		return true
	default:
		return false
	}
}

// OptimizeTable performs brute force optimization on a lookup table
func (o *BruteForceOptimizer) OptimizeTable(table *stakergs.LookupTable) (*BruteForceResult, error) {
	startTime := time.Now()
	n := len(table.Outcomes)
	if n == 0 {
		return nil, nil
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

	// Create base bucket optimizer for initial assignment
	baseOptimizer := NewBucketOptimizer(o.config)

	// Assign outcomes to buckets
	assignments, lossIndices, warnings := baseOptimizer.assignOutcomesToBuckets(payouts)

	// Calculate initial target probabilities
	probWarnings := baseOptimizer.calculateTargetProbabilities(assignments)
	warnings = append(warnings, probWarnings...)

	// Calculate initial weights using the base algorithm
	newWeights, bucketResults, lossResult, weightWarnings := baseOptimizer.calculateWeights(payouts, assignments, lossIndices)
	warnings = append(warnings, weightWarnings...)

	// Send initial progress
	o.sendProgress("init", 0, calculateRTPFromWeights(newWeights, payouts))

	// Handle global max win frequency if specified
	if o.config.GlobalMaxWinFreq > 0 {
		o.applyGlobalMaxWinFrequency(newWeights, payouts, assignments, lossIndices)
	}

	// Handle per-bucket max win frequency
	for i := range assignments {
		if assignments[i].config.Type == ConstraintMaxWinFreq && assignments[i].config.MaxWinFrequency > 0 {
			o.applyBucketMaxWinFrequency(&assignments[i], newWeights, payouts)
		}
	}

	// Phase 2: Iterative refinement using coordinate descent
	currentRTP := calculateRTPFromWeights(newWeights, payouts)
	iteration := 0
	converged := math.Abs(currentRTP-o.config.TargetRTP) <= o.config.RTPTolerance

	// Track best result to prevent overshooting
	best := &bestSearchResult{
		weights: copyWeights(newWeights),
		rtp:     currentRTP,
		error:   math.Abs(currentRTP - o.config.TargetRTP),
	}

	// Iterative search loop
	stopped := false
	for iteration < o.config.MaxIterations && !converged && !stopped {
		iteration++

		// Check for stop signal every iteration
		if o.isStopped() {
			stopped = true
			break
		}

		// Apply coordinate descent on loss weight
		if len(lossIndices) > 0 {
			newWeights = o.refineLossWeight(newWeights, payouts, lossIndices)
		}

		currentRTP = calculateRTPFromWeights(newWeights, payouts)
		currentError := math.Abs(currentRTP - o.config.TargetRTP)

		// Update best result if this is closer to target
		if currentError < best.error {
			best = &bestSearchResult{
				weights: copyWeights(newWeights),
				rtp:     currentRTP,
				error:   currentError,
			}
		}

		converged = currentError <= o.config.RTPTolerance

		// Send progress every 10 iterations or when converged
		if iteration%10 == 0 || converged {
			o.sendProgress("refine", iteration, currentRTP)
		}

		// Early exit when converged - save best and break
		if converged {
			break
		}

		// Fine-tune bucket weights if RTP is still off
		if iteration%50 == 0 && !converged {
			newWeights = o.fineTuneBucketWeights(newWeights, payouts, assignments, lossIndices)
			currentRTP = calculateRTPFromWeights(newWeights, payouts)
			currentError = math.Abs(currentRTP - o.config.TargetRTP)

			// Update best result if fine-tuning improved it
			if currentError < best.error {
				best = &bestSearchResult{
					weights: copyWeights(newWeights),
					rtp:     currentRTP,
					error:   currentError,
				}
			}

			converged = currentError <= o.config.RTPTolerance
		}
	}

	// Use best result instead of last iteration (prevents overshooting)
	finalWeights := best.weights
	finalRTP := best.rtp
	finalError := best.error
	finalConverged := finalError <= o.config.RTPTolerance

	// Final progress update with best result
	o.sendProgress("complete", iteration, finalRTP)

	// Recalculate bucket results with best weights
	bucketResults = o.recalculateBucketResults(finalWeights, payouts, assignments)

	// Recalculate loss result
	if len(lossIndices) > 0 {
		lossResult = baseOptimizer.calculateLossResult(finalWeights, payouts, lossIndices)
	}

	// Build outcome details
	outcomeDetails := baseOptimizer.buildOutcomeDetails(table, payouts, originalWeights, finalWeights, assignments, lossIndices)

	// Add convergence warning if needed
	if !finalConverged {
		diff := (finalRTP - o.config.TargetRTP) * 100
		if math.Abs(diff) > 0.1 {
			warnings = append(warnings, "Brute force did not fully converge. Consider adjusting constraints or increasing iterations.")
		}
	}

	result := &BucketOptimizerResult{
		OriginalRTP:    originalRTP,
		FinalRTP:       finalRTP,
		TargetRTP:      o.config.TargetRTP,
		Converged:      finalConverged,
		NewWeights:     finalWeights,
		BucketResults:  bucketResults,
		LossResult:     lossResult,
		TotalWeight:    sumUint64(finalWeights),
		Warnings:       warnings,
		OutcomeDetails: outcomeDetails,
	}

	return &BruteForceResult{
		BucketOptimizerResult: result,
		Iterations:            iteration,
		SearchDuration:        time.Since(startTime).Milliseconds(),
		FinalError:            finalError,
	}, nil
}

// sendProgress sends a progress update if channel is available
func (o *BruteForceOptimizer) sendProgress(phase string, iteration int, currentRTP float64) {
	if o.progressChan == nil {
		return
	}

	progress := BruteForceProgress{
		Phase:      phase,
		Iteration:  iteration,
		MaxIter:    o.config.MaxIterations,
		CurrentRTP: currentRTP,
		TargetRTP:  o.config.TargetRTP,
		Error:      math.Abs(currentRTP - o.config.TargetRTP),
		Converged:  math.Abs(currentRTP-o.config.TargetRTP) <= o.config.RTPTolerance,
	}

	select {
	case o.progressChan <- progress:
	default:
		// Channel full, skip this update
	}
}

// applyGlobalMaxWinFrequency applies global maximum win frequency constraint
func (o *BruteForceOptimizer) applyGlobalMaxWinFrequency(weights []uint64, payouts []float64, assignments []bucketAssignment, lossIndices []int) {
	// Find the global maximum payout outcome
	maxPayoutIdx := -1
	maxPayout := 0.0
	for i, p := range payouts {
		if p > maxPayout {
			maxPayout = p
			maxPayoutIdx = i
		}
	}

	if maxPayoutIdx < 0 || o.config.GlobalMaxWinFreq <= 0 {
		return
	}

	// Calculate required weight for this outcome
	// Probability = 1/GlobalMaxWinFreq
	// Weight = Probability * TotalWeight
	targetProb := 1.0 / o.config.GlobalMaxWinFreq
	totalWeight := sumUint64(weights)
	requiredWeight := uint64(targetProb * float64(totalWeight))

	if requiredWeight < o.config.MinWeight {
		requiredWeight = o.config.MinWeight
	}

	// Adjust the max payout outcome's weight
	oldWeight := weights[maxPayoutIdx]
	weights[maxPayoutIdx] = requiredWeight

	// Compensate by adjusting loss weights if possible
	if len(lossIndices) > 0 {
		// Distribute the weight change across loss outcomes
		weightDiff := int64(oldWeight) - int64(requiredWeight)
		adjustmentPerLoss := weightDiff / int64(len(lossIndices))

		for _, idx := range lossIndices {
			newLossWeight := int64(weights[idx]) + adjustmentPerLoss
			if newLossWeight < int64(o.config.MinWeight) {
				newLossWeight = int64(o.config.MinWeight)
			}
			weights[idx] = uint64(newLossWeight)
		}
	}
}

// applyBucketMaxWinFrequency applies max win frequency constraint within a bucket
func (o *BruteForceOptimizer) applyBucketMaxWinFrequency(bucket *bucketAssignment, weights []uint64, payouts []float64) {
	if len(bucket.outcomeIndices) == 0 {
		return
	}

	// Find max payout in this bucket
	maxPayoutIdx := bucket.outcomeIndices[0]
	maxPayout := payouts[maxPayoutIdx]
	for _, idx := range bucket.outcomeIndices {
		if payouts[idx] > maxPayout {
			maxPayout = payouts[idx]
			maxPayoutIdx = idx
		}
	}

	// Calculate required weight
	targetProb := 1.0 / bucket.config.MaxWinFrequency
	totalWeight := sumUint64(weights)
	requiredWeight := uint64(targetProb * float64(totalWeight))

	if requiredWeight < o.config.MinWeight {
		requiredWeight = o.config.MinWeight
	}

	weights[maxPayoutIdx] = requiredWeight
}

// refineLossWeight uses binary search to find optimal loss weight
func (o *BruteForceOptimizer) refineLossWeight(weights []uint64, payouts []float64, lossIndices []int) []uint64 {
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
	// RTP = weightedPayoutSum / (totalWinWeight + totalLossWeight)
	// targetRTP * (totalWinWeight + totalLossWeight) = weightedPayoutSum
	// totalLossWeight = weightedPayoutSum / targetRTP - totalWinWeight
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

// fineTuneBucketWeights adjusts bucket weights to improve RTP accuracy
func (o *BruteForceOptimizer) fineTuneBucketWeights(weights []uint64, payouts []float64, assignments []bucketAssignment, lossIndices []int) []uint64 {
	result := make([]uint64, len(weights))
	copy(result, weights)

	currentRTP := calculateRTPFromWeights(result, payouts)
	rtpError := currentRTP - o.config.TargetRTP

	if math.Abs(rtpError) <= o.config.RTPTolerance {
		return result
	}

	// Find the bucket with the highest RTP contribution that we can adjust
	var bestBucketIdx int = -1
	var bestRTPContrib float64 = 0

	for i, bucket := range assignments {
		if len(bucket.outcomeIndices) == 0 {
			continue
		}
		// Skip hard-constrained frequency buckets
		if bucket.config.Type == ConstraintFrequency && bucket.config.Priority != PrioritySoft {
			continue
		}

		// Calculate bucket's RTP contribution
		bucketRTP := 0.0
		totalWeight := sumUint64(result)
		for _, idx := range bucket.outcomeIndices {
			prob := float64(result[idx]) / float64(totalWeight)
			bucketRTP += prob * payouts[idx]
		}

		if bucketRTP > bestRTPContrib {
			bestRTPContrib = bucketRTP
			bestBucketIdx = i
		}
	}

	if bestBucketIdx < 0 {
		return result
	}

	// Adjust bucket weights
	bucket := assignments[bestBucketIdx]
	scaleFactor := 1.0

	if rtpError > 0 {
		// RTP too high - reduce this bucket's weight
		scaleFactor = 0.95
	} else {
		// RTP too low - increase this bucket's weight
		scaleFactor = 1.05
	}

	for _, idx := range bucket.outcomeIndices {
		newWeight := uint64(float64(result[idx]) * scaleFactor)
		if newWeight < o.config.MinWeight {
			newWeight = o.config.MinWeight
		}
		result[idx] = newWeight
	}

	return result
}

// recalculateBucketResults calculates final bucket results from weights
func (o *BruteForceOptimizer) recalculateBucketResults(weights []uint64, payouts []float64, assignments []bucketAssignment) []BucketResult {
	totalWeight := sumUint64(weights)
	results := make([]BucketResult, 0, len(assignments))

	for _, bucket := range assignments {
		if len(bucket.outcomeIndices) == 0 {
			continue
		}

		var bucketWeight uint64
		var bucketRTP float64
		var sumPayout float64

		for _, idx := range bucket.outcomeIndices {
			bucketWeight += weights[idx]
			prob := float64(weights[idx]) / float64(totalWeight)
			bucketRTP += prob * payouts[idx]
			sumPayout += payouts[idx]
		}

		actualProb := float64(bucketWeight) / float64(totalWeight)
		avgPayout := sumPayout / float64(len(bucket.outcomeIndices))

		targetFreq := 0.0
		if bucket.targetProb > 0 {
			targetFreq = 1.0 / bucket.targetProb
		}

		results = append(results, BucketResult{
			Name:              bucket.config.Name,
			MinPayout:         bucket.config.MinPayout,
			MaxPayout:         bucket.config.MaxPayout,
			OutcomeCount:      len(bucket.outcomeIndices),
			TargetProbability: bucket.targetProb,
			ActualProbability: actualProb,
			TargetFrequency:   targetFreq,
			ActualFrequency:   1.0 / actualProb,
			RTPContribution:   bucketRTP * 100,
			TotalWeight:       bucketWeight,
			AvgPayout:         avgPayout,
		})
	}

	return results
}

// GetMaxIterationsForMode returns default iterations for optimization mode
func GetMaxIterationsForMode(mode OptimizationMode) int {
	return getDefaultIterations(mode)
}

// OptimizeWithProgress runs optimization with progress reporting
func OptimizeWithProgress(table *stakergs.LookupTable, config *BucketOptimizerConfig, progressChan chan<- BruteForceProgress) (*BruteForceResult, error) {
	optimizer := NewBruteForceOptimizer(config, progressChan)
	return optimizer.OptimizeTable(table)
}

// DefaultBruteForceConfig returns default config for brute force optimization
func DefaultBruteForceConfig() *BucketOptimizerConfig {
	config := DefaultBucketConfig()
	config.EnableBruteForce = true
	config.OptimizationMode = ModeBalanced
	config.MaxIterations = getDefaultIterations(ModeBalanced)
	config.RTPTolerance = 0.0001 // 0.01% tolerance
	config.MinWeight = common.BaseWeight / 1000000000 // 1000 minimum
	return config
}
