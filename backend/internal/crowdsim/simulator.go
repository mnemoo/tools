package crowdsim

import (
	"crypto/rand"
	"fmt"
	"math/big"
	mrand "math/rand"
	"sort"
	"sync"
	"time"

	"stakergs"
)

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

// Sample returns a random outcome based on weights using math/rand.
func (ws *WeightedSampler) Sample(rng *mrand.Rand) stakergs.Outcome {
	r := rng.Uint64() % ws.totalWeight
	idx := sort.Search(len(ws.cumWeights), func(i int) bool {
		return ws.cumWeights[i] > r
	})
	return ws.outcomes[idx]
}

// SampleCrypto returns a random outcome using crypto/rand.
func (ws *WeightedSampler) SampleCrypto() stakergs.Outcome {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(ws.totalWeight)))
	r := uint64(n.Int64())

	idx := sort.Search(len(ws.cumWeights), func(i int) bool {
		return ws.cumWeights[i] > r
	})
	return ws.outcomes[idx]
}

// CrowdSimulator handles multi-player simulation.
type CrowdSimulator struct {
	sampler        *WeightedSampler
	config         SimConfig
	theoreticalRTP float64
	mode           string
	modeCost       float64 // Cost from LUT (bet amount)
}

// NewCrowdSimulator creates a new simulator for the given lookup table.
func NewCrowdSimulator(lut *stakergs.LookupTable, config SimConfig) *CrowdSimulator {
	modeCost := lut.Cost
	if modeCost <= 0 {
		modeCost = 1.0
	}

	// Use the canonical RTP calculation from stakergs:
	// RTP = rawRTP / cost, where rawRTP = avg(payout/100)
	theoreticalRTP := lut.RTP()

	// Normalize everything to unit bets:
	// - BetAmount = 1 (each spin costs 1 unit)
	// - Payout will be normalized: (outcome.Payout/100) / cost
	// - InitialBalance stays as user input (number of unit bets)
	// This way RTP = avg(payout) / 1 = avg((outcome.Payout/100)/cost) = theoreticalRTP
	config.BetAmount = 1.0

	return &CrowdSimulator{
		sampler:        NewWeightedSampler(lut),
		config:         config,
		theoreticalRTP: theoreticalRTP,
		mode:           lut.Mode,
		modeCost:       modeCost,
	}
}

// Progress reports simulation progress.
type Progress struct {
	PlayersComplete int   `json:"players_complete"`
	TotalPlayers    int   `json:"total_players"`
	PercentComplete int   `json:"percent_complete"`
	ElapsedMs       int64 `json:"elapsed_ms"`
}

// Run executes the full simulation sequentially.
func (s *CrowdSimulator) Run(progressCallback func(Progress)) *SimResult {
	start := time.Now()

	trackHistory := !s.config.StreamingMode
	players := make([]*Player, s.config.PlayerCount)

	// Create a single RNG for sequential mode
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))

	for i := 0; i < s.config.PlayerCount; i++ {
		player := NewPlayer(i, s.config.InitialBalance, trackHistory, s.config.SpinsPerSession)

		// Run session
		for spin := 0; spin < s.config.SpinsPerSession; spin++ {
			var payout float64
			if s.config.UseCryptoRNG {
				outcome := s.sampler.SampleCrypto()
				// Payout from LUT is multiplier * 100 (e.g., 150 = 1.5x of base bet)
				// Normalize by cost to get multiplier relative to mode cost
				// Example: bonus cost=350, payout=34055 -> 340.55 / 350 = 0.973x
				payout = float64(outcome.Payout) / 100.0 / s.modeCost
			} else {
				outcome := s.sampler.Sample(rng)
				// Payout from LUT is multiplier * 100 (e.g., 150 = 1.5x of base bet)
				// Normalize by cost to get multiplier relative to mode cost
				payout = float64(outcome.Payout) / 100.0 / s.modeCost
			}

			player.ProcessSpin(spin, payout, s.config.BetAmount, s.config.BigWinThreshold, s.config.DangerThreshold)
		}

		players[i] = player

		// Report progress every 100 players
		if progressCallback != nil && (i+1)%100 == 0 {
			progressCallback(Progress{
				PlayersComplete: i + 1,
				TotalPlayers:    s.config.PlayerCount,
				PercentComplete: (i + 1) * 100 / s.config.PlayerCount,
				ElapsedMs:       time.Since(start).Milliseconds(),
			})
		}
	}

	return s.calculateResults(players, time.Since(start))
}

// RunParallel executes simulation with parallel workers.
func (s *CrowdSimulator) RunParallel(progressCallback func(Progress)) *SimResult {
	start := time.Now()

	trackHistory := !s.config.StreamingMode
	players := make([]*Player, s.config.PlayerCount)

	// Worker pool
	playerChan := make(chan int, s.config.PlayerCount)
	var wg sync.WaitGroup

	// Progress tracking
	var progressMu sync.Mutex
	completed := 0

	// Start workers
	for w := 0; w < s.config.ParallelWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each worker has its own RNG
			rng := mrand.New(mrand.NewSource(time.Now().UnixNano() + int64(mrand.Intn(1000000))))

			for playerID := range playerChan {
				player := NewPlayer(playerID, s.config.InitialBalance, trackHistory, s.config.SpinsPerSession)

				// Run session
				for spin := 0; spin < s.config.SpinsPerSession; spin++ {
					var payout float64
					if s.config.UseCryptoRNG {
						outcome := s.sampler.SampleCrypto()
						// Payout from LUT is multiplier * 100 (e.g., 150 = 1.5x of base bet)
						// Normalize by cost to get multiplier relative to mode cost
						payout = float64(outcome.Payout) / 100.0 / s.modeCost
					} else {
						outcome := s.sampler.Sample(rng)
						// Payout from LUT is multiplier * 100 (e.g., 150 = 1.5x of base bet)
						// Normalize by cost to get multiplier relative to mode cost
						payout = float64(outcome.Payout) / 100.0 / s.modeCost
					}

					player.ProcessSpin(spin, payout, s.config.BetAmount, s.config.BigWinThreshold, s.config.DangerThreshold)
				}

				players[playerID] = player

				// Update progress
				if progressCallback != nil {
					progressMu.Lock()
					completed++
					if completed%100 == 0 {
						progressCallback(Progress{
							PlayersComplete: completed,
							TotalPlayers:    s.config.PlayerCount,
							PercentComplete: completed * 100 / s.config.PlayerCount,
							ElapsedMs:       time.Since(start).Milliseconds(),
						})
					}
					progressMu.Unlock()
				}
			}
		}()
	}

	// Send work
	for i := 0; i < s.config.PlayerCount; i++ {
		playerChan <- i
	}
	close(playerChan)

	// Wait for completion
	wg.Wait()

	return s.calculateResults(players, time.Since(start))
}

// calculateResults computes all metrics from player data.
func (s *CrowdSimulator) calculateResults(players []*Player, duration time.Duration) *SimResult {
	isBonusMode := s.modeCost > 1.5
	note := "Standard mode. Payouts are shown as bet multipliers."
	if isBonusMode {
		note = fmt.Sprintf("Bonus mode (cost=%.0fx). Payouts are normalized: a %.0fx absolute payout = 1.0x normalized.", s.modeCost, s.modeCost)
	}

	result := &SimResult{
		Mode: s.mode,
		ModeInfo: ModeInfo{
			Cost:        s.modeCost,
			IsBonusMode: isBonusMode,
			Note:        note,
		},
		Config:         s.config,
		DurationMs:     duration.Milliseconds(),
		TheoreticalRTP: round4(s.theoreticalRTP),
	}

	// Calculate actual RTP
	var totalWagered, totalWon float64
	for _, p := range players {
		totalWagered += p.TotalWagered
		totalWon += p.TotalWon
	}
	result.ActualRTP = round4(totalWon / totalWagered)
	result.RTPDeviation = round4(result.ActualRTP - result.TheoreticalRTP)

	// Calculate all metrics
	result.FinalPoP = CalcPoP(players)
	result.PoPCurve = CalcPoPCurve(players, s.config.SpinsPerSession, s.config.InitialBalance)
	result.BalanceCurve = CalcBalanceCurve(players, s.config.SpinsPerSession)
	result.BalanceStats = CalcBalanceStats(players)
	result.PeakStats = CalcPeakStats(players)
	result.DrawdownStats = CalcDrawdownStats(players)
	result.DangerStats = CalcDangerStats(players)
	result.StreakStats = CalcStreakStats(players)
	result.BigWinStats = CalcBigWinStats(players)
	result.VolatilityProfile = ClassifyVolatility(result.FinalPoP, result.BalanceStats, result.PeakStats, s.config.InitialBalance)
	result.CompositeScore = CalcCompositeScore(result, DefaultRankingWeights(), s.config.InitialBalance)

	// Player summaries (limit to avoid huge responses)
	if !s.config.StreamingMode && len(players) <= 1000 {
		result.PlayerSummaries = make([]PlayerSummary, len(players))
		for i, p := range players {
			result.PlayerSummaries[i] = p.Summary()
		}
	}

	return result
}
