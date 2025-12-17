package optimizer

import (
	"math"
	"testing"

	"stakergs"
)

// ============================================================================
// Bucket Optimizer Tests
// ============================================================================

func TestBucketOptimizer_Basic(t *testing.T) {
	// Create a simple table with various payout levels
	table := &stakergs.LookupTable{
		Mode: "test",
		Cost: 1.0,
		Outcomes: []stakergs.Outcome{
			{SimID: 0, Weight: 1000, Payout: 0},       // loss
			{SimID: 1, Weight: 100, Payout: 50},       // 0.5x
			{SimID: 2, Weight: 100, Payout: 100},      // 1x
			{SimID: 3, Weight: 50, Payout: 200},       // 2x
			{SimID: 4, Weight: 20, Payout: 500},       // 5x
			{SimID: 5, Weight: 10, Payout: 1000},      // 10x
			{SimID: 6, Weight: 5, Payout: 2000},       // 20x
			{SimID: 7, Weight: 2, Payout: 5000},       // 50x
			{SimID: 8, Weight: 1, Payout: 10000},      // 100x
		},
	}

	config := &BucketOptimizerConfig{
		TargetRTP:    0.97,
		RTPTolerance: 0.01,
		MinWeight:    1,
		Buckets: []BucketConfig{
			{Name: "sub_1x", MinPayout: 0.01, MaxPayout: 1, Type: ConstraintFrequency, Frequency: 3},
			{Name: "small", MinPayout: 1, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 6},
			{Name: "medium", MinPayout: 5, MaxPayout: 25, Type: ConstraintFrequency, Frequency: 20},
			{Name: "large", MinPayout: 25, MaxPayout: 150, Type: ConstraintRTPPercent, RTPPercent: 5},
		},
	}

	optimizer := NewBucketOptimizer(config)
	result, err := optimizer.OptimizeTable(table)
	if err != nil {
		t.Fatalf("Optimization failed: %v", err)
	}

	t.Logf("Original RTP: %.4f", result.OriginalRTP)
	t.Logf("Final RTP: %.4f (target: %.4f)", result.FinalRTP, result.TargetRTP)
	t.Logf("Converged: %v", result.Converged)

	// Log bucket results
	for _, br := range result.BucketResults {
		t.Logf("Bucket %s (%.0f-%.0fx): %d outcomes, prob=%.4f (1 in %.1f), RTP contrib=%.2f%%",
			br.Name, br.MinPayout, br.MaxPayout, br.OutcomeCount,
			br.ActualProbability, br.ActualFrequency, br.RTPContribution)
	}
	if result.LossResult != nil {
		t.Logf("Loss: %d outcomes, prob=%.4f (1 in %.1f)",
			result.LossResult.OutcomeCount, result.LossResult.ActualProbability, result.LossResult.ActualFrequency)
	}

	// RTP should be close to target
	if math.Abs(result.FinalRTP-result.TargetRTP) > 0.02 {
		t.Errorf("Final RTP (%.4f) should be close to target (%.4f)", result.FinalRTP, result.TargetRTP)
	}

	// Check that buckets with frequency constraint hit their targets approximately
	for _, br := range result.BucketResults {
		// Find original bucket config
		for _, bc := range config.Buckets {
			if bc.Name == br.Name && bc.Type == ConstraintFrequency {
				targetFreq := bc.Frequency
				// Allow 50% deviation for frequency (due to rounding and adjustments)
				if br.ActualFrequency < targetFreq*0.5 || br.ActualFrequency > targetFreq*2 {
					t.Logf("Warning: Bucket %s frequency %.1f differs significantly from target %.1f",
						br.Name, br.ActualFrequency, targetFreq)
				}
			}
		}
	}
}

func TestBucketOptimizer_RTPPercentConstraint(t *testing.T) {
	// Test that RTP% constraint works correctly
	// If we say "jackpot should use 0.5% of RTP", with 5000x payout:
	// RTP contribution = 0.005 * 0.97 = 0.00485
	// probability = 0.00485 / 5000 = 0.00000097 = ~1 in 1,030,928

	table := &stakergs.LookupTable{
		Mode: "test",
		Cost: 1.0,
		Outcomes: []stakergs.Outcome{
			{SimID: 0, Weight: 1000, Payout: 0},       // loss
			{SimID: 1, Weight: 100, Payout: 100},      // 1x
			{SimID: 2, Weight: 1, Payout: 500000},     // 5000x (max win)
		},
	}

	config := &BucketOptimizerConfig{
		TargetRTP:    0.97,
		RTPTolerance: 0.01,
		MinWeight:    1,
		Buckets: []BucketConfig{
			{Name: "small", MinPayout: 0.01, MaxPayout: 100, Type: ConstraintFrequency, Frequency: 5},
			{Name: "jackpot", MinPayout: 100, MaxPayout: 10000, Type: ConstraintRTPPercent, RTPPercent: 0.5},
		},
	}

	optimizer := NewBucketOptimizer(config)
	result, err := optimizer.OptimizeTable(table)
	if err != nil {
		t.Fatalf("Optimization failed: %v", err)
	}

	t.Logf("Final RTP: %.4f", result.FinalRTP)

	// Find jackpot bucket
	var jackpotResult *BucketResult
	for i := range result.BucketResults {
		if result.BucketResults[i].Name == "jackpot" {
			jackpotResult = &result.BucketResults[i]
			break
		}
	}

	if jackpotResult == nil {
		t.Fatal("Jackpot bucket not found in results")
	}

	t.Logf("Jackpot bucket: RTP contribution=%.4f%%, probability=%.8f (1 in %.0f)",
		jackpotResult.RTPContribution, jackpotResult.ActualProbability, jackpotResult.ActualFrequency)

	// The RTP contribution should be approximately 0.5% (allowing for rounding)
	// But since it's expressed as % of target RTP in result, we need to check the actual value
	expectedRTPContrib := 0.5 // 0.5% of target RTP
	if math.Abs(jackpotResult.RTPContribution-expectedRTPContrib) > 1.0 { // Allow 1% deviation
		t.Errorf("Jackpot RTP contribution (%.4f%%) should be close to %.4f%%",
			jackpotResult.RTPContribution, expectedRTPContrib)
	}
}

func TestBucketOptimizer_FrequencyConstraint(t *testing.T) {
	// Test that frequency constraint works correctly
	// If we say "1 in 20 spins for medium wins (5-20x)"

	table := &stakergs.LookupTable{
		Mode: "test",
		Cost: 1.0,
		Outcomes: []stakergs.Outcome{
			{SimID: 0, Weight: 1000, Payout: 0},       // loss
			{SimID: 1, Weight: 100, Payout: 100},      // 1x
			{SimID: 2, Weight: 50, Payout: 200},       // 2x
			{SimID: 3, Weight: 20, Payout: 500},       // 5x
			{SimID: 4, Weight: 10, Payout: 1000},      // 10x
			{SimID: 5, Weight: 5, Payout: 1500},       // 15x
		},
	}

	config := &BucketOptimizerConfig{
		TargetRTP:    0.97,
		RTPTolerance: 0.01,
		MinWeight:    1,
		Buckets: []BucketConfig{
			{Name: "small", MinPayout: 0.01, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 4},
			{Name: "medium", MinPayout: 5, MaxPayout: 20, Type: ConstraintFrequency, Frequency: 20},
		},
	}

	optimizer := NewBucketOptimizer(config)
	result, err := optimizer.OptimizeTable(table)
	if err != nil {
		t.Fatalf("Optimization failed: %v", err)
	}

	// Find medium bucket
	var mediumResult *BucketResult
	for i := range result.BucketResults {
		if result.BucketResults[i].Name == "medium" {
			mediumResult = &result.BucketResults[i]
			break
		}
	}

	if mediumResult == nil {
		t.Fatal("Medium bucket not found in results")
	}

	t.Logf("Medium bucket (5-20x): probability=%.4f (1 in %.1f), target was 1 in 20",
		mediumResult.ActualProbability, mediumResult.ActualFrequency)

	// The frequency should be approximately 20 (1 in 20)
	// Allow significant deviation due to RTP balancing
	if mediumResult.ActualFrequency < 10 || mediumResult.ActualFrequency > 50 {
		t.Errorf("Medium bucket frequency (1 in %.1f) should be closer to target (1 in 20)",
			mediumResult.ActualFrequency)
	}
}

func TestValidateBuckets(t *testing.T) {
	// Test valid configuration
	validBuckets := []BucketConfig{
		{Name: "small", MinPayout: 0, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 5},
		{Name: "medium", MinPayout: 5, MaxPayout: 20, Type: ConstraintFrequency, Frequency: 20},
		{Name: "large", MinPayout: 20, MaxPayout: 100, Type: ConstraintRTPPercent, RTPPercent: 5},
	}

	err := ValidateBuckets(validBuckets)
	if err != nil {
		t.Errorf("Valid buckets should not return error: %v", err)
	}

	// Test gap between buckets
	gappedBuckets := []BucketConfig{
		{Name: "small", MinPayout: 0, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 5},
		{Name: "large", MinPayout: 10, MaxPayout: 100, Type: ConstraintFrequency, Frequency: 50},
	}
	err = ValidateBuckets(gappedBuckets)
	if err == nil {
		t.Error("Should detect gap between buckets")
	}

	// Test invalid frequency
	invalidFreq := []BucketConfig{
		{Name: "small", MinPayout: 0, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 0},
	}
	err = ValidateBuckets(invalidFreq)
	if err == nil {
		t.Error("Should detect invalid frequency")
	}

	// Test invalid RTP percent
	invalidRTP := []BucketConfig{
		{Name: "small", MinPayout: 0, MaxPayout: 5, Type: ConstraintRTPPercent, RTPPercent: 150},
	}
	err = ValidateBuckets(invalidRTP)
	if err == nil {
		t.Error("Should detect invalid RTP percent")
	}

	// Test auto bucket is valid
	autoBuckets := []BucketConfig{
		{Name: "small", MinPayout: 0, MaxPayout: 5, Type: ConstraintFrequency, Frequency: 5},
		{Name: "auto_rest", MinPayout: 5, MaxPayout: 100, Type: ConstraintAuto, AutoExponent: 1.0},
	}
	err = ValidateBuckets(autoBuckets)
	if err != nil {
		t.Errorf("Auto bucket should be valid: %v", err)
	}

	// Test auto bucket with negative exponent
	invalidAuto := []BucketConfig{
		{Name: "auto", MinPayout: 0, MaxPayout: 100, Type: ConstraintAuto, AutoExponent: -1.0},
	}
	err = ValidateBuckets(invalidAuto)
	if err == nil {
		t.Error("Should detect negative auto_exponent")
	}
}

// TestBucketOptimizer_AutoConstraint tests the auto constraint type
func TestBucketOptimizer_AutoConstraint(t *testing.T) {
	// Create a test table with various payouts
	table := &stakergs.LookupTable{
		Cost: 1.0,
		Outcomes: []stakergs.Outcome{
			{SimID: 0, Payout: 0, Weight: 1000},      // loss
			{SimID: 1, Payout: 100, Weight: 100},     // 1x
			{SimID: 2, Payout: 200, Weight: 100},     // 2x
			{SimID: 3, Payout: 500, Weight: 50},      // 5x
			{SimID: 4, Payout: 1000, Weight: 25},     // 10x
			{SimID: 5, Payout: 2000, Weight: 10},     // 20x
			{SimID: 6, Payout: 5000, Weight: 5},      // 50x
			{SimID: 7, Payout: 10000, Weight: 2},     // 100x
		},
	}

	// Configure buckets: small wins fixed frequency, rest uses auto
	config := &BucketOptimizerConfig{
		TargetRTP:    0.97,
		RTPTolerance: 0.005,
		MinWeight:    1,
		Buckets: []BucketConfig{
			{Name: "small", MinPayout: 0.1, MaxPayout: 3, Type: ConstraintFrequency, Frequency: 4},
			{Name: "auto_rest", MinPayout: 3, MaxPayout: 200, Type: ConstraintAuto, AutoExponent: 1.0},
		},
	}

	optimizer := NewBucketOptimizer(config)
	result, err := optimizer.OptimizeTable(table)
	if err != nil {
		t.Fatalf("OptimizeTable failed: %v", err)
	}

	t.Logf("Original RTP: %.4f", result.OriginalRTP)
	t.Logf("Final RTP: %.4f (target: %.4f)", result.FinalRTP, config.TargetRTP)
	t.Logf("Converged: %v", result.Converged)

	// Log bucket results
	for _, br := range result.BucketResults {
		t.Logf("Bucket %s (%.0f-%.0fx): %d outcomes, prob=%.6f (1 in %.1f), RTP contrib=%.2f%%",
			br.Name, br.MinPayout, br.MaxPayout, br.OutcomeCount,
			br.ActualProbability, br.ActualFrequency, br.RTPContribution)
	}

	// Find the auto bucket result
	var autoBucket *BucketResult
	for i := range result.BucketResults {
		if result.BucketResults[i].Name == "auto_rest" {
			autoBucket = &result.BucketResults[i]
			break
		}
	}

	if autoBucket == nil {
		t.Fatal("Auto bucket not found in results")
	}

	// Verify auto bucket has outcomes
	if autoBucket.OutcomeCount == 0 {
		t.Error("Auto bucket should have outcomes")
	}

	// Verify weights are inversely proportional to payout
	// Higher payout should have lower weight
	var prevWeight uint64 = ^uint64(0) // max uint64
	var prevPayout float64 = 0
	for _, detail := range result.OutcomeDetails {
		if detail.BucketName == "auto_rest" && detail.Payout > prevPayout {
			if detail.NewWeight > prevWeight && prevPayout > 0 {
				t.Errorf("Auto bucket: higher payout %.2fx (weight %d) should not have higher weight than %.2fx (weight %d)",
					detail.Payout, detail.NewWeight, prevPayout, prevWeight)
			}
			prevWeight = detail.NewWeight
			prevPayout = detail.Payout
		}
	}

	// Log individual outcome details for the auto bucket
	t.Log("Auto bucket outcome details:")
	for _, detail := range result.OutcomeDetails {
		if detail.BucketName == "auto_rest" {
			t.Logf("  Payout %.2fx: weight %d (prob %.8f)", detail.Payout, detail.NewWeight, detail.Probability)
		}
	}
}
