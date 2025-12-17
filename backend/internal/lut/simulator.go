package lut

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"stakergs"
)

// Simulator handles weighted random spin simulations.
type Simulator struct {
	rng *rand.Rand
	mu  sync.Mutex
}

// NewSimulator creates a new simulator with a seeded RNG.
func NewSimulator() *Simulator {
	return &Simulator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SimulationConfig holds parameters for a simulation run.
type SimulationConfig struct {
	Spins       int       `json:"spins"`        // Number of spins per trial
	Trials      int       `json:"trials"`       // Number of trials to run
	Bet         float64   `json:"bet"`          // Bet amount per spin
	TargetRTP   float64   `json:"target_rtp"`   // Target RTP threshold (e.g., 0.97)
	TestSpins   []int     `json:"test_spins"`   // Spin counts to test RTP at
	TestWeights []float64 `json:"test_weights"` // Weights for each test spin (for scoring)
}

// SimulationResult holds the results of a simulation run.
type SimulationResult struct {
	Mode           string          `json:"mode"`
	Config         SimulationConfig `json:"config"`
	TotalSpins     int             `json:"total_spins"`
	TotalWagered   float64         `json:"total_wagered"`
	TotalWon       float64         `json:"total_won"`
	ActualRTP      float64         `json:"actual_rtp"`
	HitCount       int             `json:"hit_count"`
	HitRate        float64         `json:"hit_rate"`
	BigWins        int             `json:"big_wins"`    // wins >= 10x
	MegaWins       int             `json:"mega_wins"`   // wins >= 50x
	MaxWin         float64         `json:"max_win"`
	SpinResults    []SpinResult    `json:"spin_results,omitempty"`
	TrialSummaries []TrialSummary  `json:"trial_summaries,omitempty"`
	RTPatSpins     []RTPAtSpin     `json:"rtp_at_spins,omitempty"`
	FinalScore     float64         `json:"final_score,omitempty"`
	DurationMs     int64           `json:"duration_ms"`
}

// SpinResult holds the result of a single spin.
type SpinResult struct {
	SpinNum     int     `json:"spin_num"`
	SimID       int     `json:"sim_id"`
	Payout      float64 `json:"payout"`
	Balance     float64 `json:"balance"`
	RunningRTP  float64 `json:"running_rtp"`
}

// TrialSummary holds summary stats for a single trial.
type TrialSummary struct {
	Trial      int     `json:"trial"`
	TotalWon   float64 `json:"total_won"`
	RTP        float64 `json:"rtp"`
	HitCount   int     `json:"hit_count"`
	MaxWin     float64 `json:"max_win"`
	PassedRTP  bool    `json:"passed_rtp"`
}

// RTPAtSpin holds RTP success rate at a specific spin count.
type RTPAtSpin struct {
	SpinCount   int     `json:"spin_count"`
	SuccessRate float64 `json:"success_rate"`
	Weight      float64 `json:"weight,omitempty"`
}

// WeightedSampler provides efficient weighted random sampling.
type WeightedSampler struct {
	outcomes    []stakergs.Outcome
	cumWeights  []uint64
	totalWeight uint64
}

// NewWeightedSampler creates a sampler from a lookup table.
func NewWeightedSampler(lut *stakergs.LookupTable) *WeightedSampler {
	ws := &WeightedSampler{
		outcomes:   lut.Outcomes,
		cumWeights: make([]uint64, len(lut.Outcomes)),
	}

	var cumulative uint64
	for i, o := range lut.Outcomes {
		cumulative += o.Weight
		ws.cumWeights[i] = cumulative
	}
	ws.totalWeight = cumulative

	return ws
}

// Sample returns a random outcome based on weights.
func (ws *WeightedSampler) Sample(rng *rand.Rand) stakergs.Outcome {
	// Generate random value in [0, totalWeight)
	r := rng.Uint64() % ws.totalWeight

	// Binary search for the outcome
	idx := sort.Search(len(ws.cumWeights), func(i int) bool {
		return ws.cumWeights[i] > r
	})

	return ws.outcomes[idx]
}

// SampleWithNewRNG returns a random outcome using a fresh RNG.
func (ws *WeightedSampler) SampleWithNewRNG() stakergs.Outcome {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	r := rng.Uint64() % ws.totalWeight

	// Binary search for the outcome
	idx := sort.Search(len(ws.cumWeights), func(i int) bool {
		return ws.cumWeights[i] > r
	})

	return ws.outcomes[idx]
}

// BiasedWeightedSampler provides weighted sampling with a payout bias.
// Bias > 0 increases probability of higher payouts.
type BiasedWeightedSampler struct {
	outcomes    []stakergs.Outcome
	cumWeights  []float64
	totalWeight float64
}

// NewBiasedWeightedSampler creates a sampler with payout-based bias.
// bias = 0: normal sampling
// bias > 0: favors higher payouts AND reduces zero payouts (losses)
// bias < 0: favors lower payouts AND increases zero payouts (losses)
//
// Formula:
//   - For payout = 0: weight * 0.5^bias (positive bias reduces losses)
//   - For payout > 0: weight * (1 + payout/100)^bias
//
// Examples with bias = +5:
//   - 0x payout: 0.5^5 = 0.03 (97% fewer losses!)
//   - 1x payout: 2^5 = 32x more likely
//   - 10x payout: 11^5 = 161051x more likely
func NewBiasedWeightedSampler(lut *stakergs.LookupTable, bias float64) *BiasedWeightedSampler {
	bws := &BiasedWeightedSampler{
		outcomes:   lut.Outcomes,
		cumWeights: make([]float64, len(lut.Outcomes)),
	}

	var cumulative float64
	for i, o := range lut.Outcomes {
		var biasedWeight float64
		if o.Payout == 0 {
			// For zero payouts (losses): use 0.5 as base
			// Positive bias reduces losses, negative bias increases them
			biasedWeight = float64(o.Weight) * math.Pow(0.5, bias)
		} else {
			// For winning payouts: use (1 + payout/100) as base
			payoutMultiplier := 1.0 + float64(o.Payout)/100.0
			biasedWeight = float64(o.Weight) * math.Pow(payoutMultiplier, bias)
		}
		cumulative += biasedWeight
		bws.cumWeights[i] = cumulative
	}
	bws.totalWeight = cumulative

	return bws
}

// SampleWithNewRNG returns a random outcome using a fresh RNG.
func (bws *BiasedWeightedSampler) SampleWithNewRNG() stakergs.Outcome {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	r := rng.Float64() * bws.totalWeight

	// Binary search for the outcome
	idx := sort.Search(len(bws.cumWeights), func(i int) bool {
		return bws.cumWeights[i] > r
	})

	if idx >= len(bws.outcomes) {
		idx = len(bws.outcomes) - 1
	}

	return bws.outcomes[idx]
}

// RunSimulation executes a full simulation with multiple trials.
func (s *Simulator) RunSimulation(lut *stakergs.LookupTable, config SimulationConfig) *SimulationResult {
	start := time.Now()

	sampler := NewWeightedSampler(lut)

	result := &SimulationResult{
		Mode:   lut.Mode,
		Config: config,
	}

	// Initialize test spin success counters
	testSpinSuccess := make([]int, len(config.TestSpins))

	var totalWon float64
	var totalHits int
	var totalBigWins int
	var totalMegaWins int
	var maxWin float64

	trialSummaries := make([]TrialSummary, config.Trials)

	// Run all trials
	for trial := 0; trial < config.Trials; trial++ {
		s.mu.Lock()
		trialRNG := rand.New(rand.NewSource(s.rng.Int63()))
		s.mu.Unlock()

		var trialWon float64
		var trialHits int
		var trialMaxWin float64
		spinPayouts := make([]float64, config.Spins)

		// Run spins for this trial
		for spin := 0; spin < config.Spins; spin++ {
			outcome := sampler.Sample(trialRNG)
			payout := float64(outcome.Payout) / 100.0

			spinPayouts[spin] = payout
			trialWon += payout

			if payout > 0 {
				trialHits++
				totalHits++
			}
			if payout >= 10.0 {
				totalBigWins++
			}
			if payout >= 50.0 {
				totalMegaWins++
			}
			if payout > trialMaxWin {
				trialMaxWin = payout
			}
			if payout > maxWin {
				maxWin = payout
			}
		}

		totalWon += trialWon
		trialRTP := trialWon / (float64(config.Spins) * config.Bet)

		// Check RTP at test spin points
		for i, testSpin := range config.TestSpins {
			if testSpin <= config.Spins {
				var sumToSpin float64
				for j := 0; j < testSpin; j++ {
					sumToSpin += spinPayouts[j]
				}
				rtpAtSpin := sumToSpin / (float64(testSpin) * config.Bet)
				if rtpAtSpin >= config.TargetRTP {
					testSpinSuccess[i]++
				}
			}
		}

		trialSummaries[trial] = TrialSummary{
			Trial:     trial + 1,
			TotalWon:  round2(trialWon),
			RTP:       round4(trialRTP),
			HitCount:  trialHits,
			MaxWin:    round2(trialMaxWin),
			PassedRTP: trialRTP >= config.TargetRTP,
		}
	}

	totalSpins := config.Spins * config.Trials
	totalWagered := float64(totalSpins) * config.Bet

	result.TotalSpins = totalSpins
	result.TotalWagered = round2(totalWagered)
	result.TotalWon = round2(totalWon)
	result.ActualRTP = round4(totalWon / totalWagered)
	result.HitCount = totalHits
	result.HitRate = round4(float64(totalHits) / float64(totalSpins))
	result.BigWins = totalBigWins
	result.MegaWins = totalMegaWins
	result.MaxWin = round2(maxWin)

	// Only include trial summaries if trials <= 100
	if config.Trials <= 100 {
		result.TrialSummaries = trialSummaries
	}

	// Calculate RTP success rates at test spins
	rtpAtSpins := make([]RTPAtSpin, len(config.TestSpins))
	var finalScore float64
	for i, testSpin := range config.TestSpins {
		successRate := float64(testSpinSuccess[i]) / float64(config.Trials)
		weight := 1.0
		if i < len(config.TestWeights) {
			weight = config.TestWeights[i]
		}
		rtpAtSpins[i] = RTPAtSpin{
			SpinCount:   testSpin,
			SuccessRate: round4(successRate),
			Weight:      weight,
		}
		finalScore += successRate * weight
	}
	result.RTPatSpins = rtpAtSpins
	result.FinalScore = round4(finalScore)

	result.DurationMs = time.Since(start).Milliseconds()

	return result
}

// RunQuickSimulation runs a simple simulation and returns spin-by-spin results.
func (s *Simulator) RunQuickSimulation(lut *stakergs.LookupTable, spins int, bet float64) *SimulationResult {
	start := time.Now()

	sampler := NewWeightedSampler(lut)

	s.mu.Lock()
	rng := rand.New(rand.NewSource(s.rng.Int63()))
	s.mu.Unlock()

	spinResults := make([]SpinResult, spins)
	var totalWon float64
	var hitCount int
	var bigWins int
	var megaWins int
	var maxWin float64

	for i := 0; i < spins; i++ {
		outcome := sampler.Sample(rng)
		payout := float64(outcome.Payout) / 100.0

		totalWon += payout

		if payout > 0 {
			hitCount++
		}
		if payout >= 10.0 {
			bigWins++
		}
		if payout >= 50.0 {
			megaWins++
		}
		if payout > maxWin {
			maxWin = payout
		}

		wageredSoFar := float64(i+1) * bet
		spinResults[i] = SpinResult{
			SpinNum:    i + 1,
			SimID:      outcome.SimID,
			Payout:     round2(payout),
			Balance:    round2(totalWon - wageredSoFar),
			RunningRTP: round4(totalWon / wageredSoFar),
		}
	}

	totalWagered := float64(spins) * bet

	return &SimulationResult{
		Mode:         lut.Mode,
		TotalSpins:   spins,
		TotalWagered: round2(totalWagered),
		TotalWon:     round2(totalWon),
		ActualRTP:    round4(totalWon / totalWagered),
		HitCount:     hitCount,
		HitRate:      round4(float64(hitCount) / float64(spins)),
		BigWins:      bigWins,
		MegaWins:     megaWins,
		MaxWin:       round2(maxWin),
		SpinResults:  spinResults,
		DurationMs:   time.Since(start).Milliseconds(),
		Config: SimulationConfig{
			Spins: spins,
			Bet:   bet,
		},
	}
}
