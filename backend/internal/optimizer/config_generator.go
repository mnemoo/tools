package optimizer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
)

// PlayerProfile defines a volatility/playstyle preset
type PlayerProfile string

const (
	ProfileLowVol    PlayerProfile = "low_volatility"    // Frequent small wins
	ProfileMediumVol PlayerProfile = "medium_volatility" // Balanced
	ProfileHighVol   PlayerProfile = "high_volatility"   // Rare big wins
)

// ProfileDescription provides human-readable info about each profile
var ProfileDescriptions = map[PlayerProfile]string{
	ProfileLowVol:    "Frequent small wins, minimal risk. Ideal for casual players.",
	ProfileMediumVol: "Balanced distribution between small and large wins.",
	ProfileHighVol:   "Rare but large wins. For thrill seekers.",
}

// GeneratedConfig represents a generated bucket configuration
type GeneratedConfig struct {
	Profile     PlayerProfile  `json:"profile"`
	ProfileName string         `json:"profile_name"`
	Description string         `json:"description"`
	TargetRTP   float64        `json:"target_rtp"`
	MaxWin      float64        `json:"max_win"`
	Buckets     []BucketConfig `json:"buckets"`
	B64Config   string         `json:"b64_config"`
	Stats       ConfigStats    `json:"stats"`
}

// ConfigStats provides statistical info about the generated config
type ConfigStats struct {
	TotalBuckets    int                `json:"total_buckets"`
	RTPDistribution map[string]float64 `json:"rtp_distribution"` // bucket range -> % of RTP
	AvgHitRate      float64            `json:"avg_hit_rate"`     // Average 1 in N for any win
	MaxWinFreq      float64            `json:"max_win_freq"`     // Frequency of max win bucket
}

// ConfigGeneratorRequest contains input for config generation
type ConfigGeneratorRequest struct {
	TargetRTP float64       `json:"target_rtp"` // e.g., 0.96
	MaxWin    float64       `json:"max_win"`    // e.g., 5000
	Profile   PlayerProfile `json:"profile"`    // Optional: specific profile
}

// ConfigGeneratorResponse contains generated configs
type ConfigGeneratorResponse struct {
	Configs []GeneratedConfig `json:"configs"`
}

// ShortConfig is the compact b64 format for frontend
type ShortConfig struct {
	R int       `json:"r"` // RTP * 100 (e.g., 96 for 96%)
	B [][]any   `json:"b"` // [[min, max, type(0/1/2), value], ...]
}

// ConfigGenerator generates optimal bucket configurations
type ConfigGenerator struct{}

// NewConfigGenerator creates a new config generator
func NewConfigGenerator() *ConfigGenerator {
	return &ConfigGenerator{}
}

// GenerateAllProfiles generates configs for all profiles
func (g *ConfigGenerator) GenerateAllProfiles(targetRTP, maxWin float64) *ConfigGeneratorResponse {
	profiles := []PlayerProfile{
		ProfileLowVol,
		ProfileMediumVol,
		ProfileHighVol,
	}

	response := &ConfigGeneratorResponse{
		Configs: make([]GeneratedConfig, 0, len(profiles)),
	}

	for _, profile := range profiles {
		config := g.GenerateConfig(targetRTP, maxWin, profile)
		response.Configs = append(response.Configs, *config)
	}

	return response
}

// GenerateConfig generates a config for a specific profile
func (g *ConfigGenerator) GenerateConfig(targetRTP, maxWin float64, profile PlayerProfile) *GeneratedConfig {
	// Define bucket boundaries based on maxWin
	boundaries := g.calculateBucketBoundaries(maxWin)

	// Get RTP distribution for profile
	rtpDistribution := g.getRTPDistribution(profile, len(boundaries)-1)

	// Generate buckets
	buckets := g.generateBuckets(boundaries, rtpDistribution, targetRTP, profile)

	// Calculate b64 config
	b64Config := g.toB64Config(targetRTP, buckets)

	// Calculate stats
	stats := g.calculateStats(buckets, rtpDistribution, targetRTP)

	return &GeneratedConfig{
		Profile:     profile,
		ProfileName: g.getProfileName(profile),
		Description: ProfileDescriptions[profile],
		TargetRTP:   targetRTP,
		MaxWin:      maxWin,
		Buckets:     buckets,
		B64Config:   b64Config,
		Stats:       stats,
	}
}

// calculateBucketBoundaries determines bucket ranges based on max win
func (g *ConfigGenerator) calculateBucketBoundaries(maxWin float64) []float64 {
	// Standard boundaries, adjusted for max win
	boundaries := []float64{0}

	// Sub-1x (partial wins)
	if maxWin >= 1 {
		boundaries = append(boundaries, 1)
	}

	// 1-2x (break-even zone)
	if maxWin >= 2 {
		boundaries = append(boundaries, 2)
	}

	// 2-5x (small wins)
	if maxWin >= 5 {
		boundaries = append(boundaries, 5)
	}

	// 5-10x (medium-small)
	if maxWin >= 10 {
		boundaries = append(boundaries, 10)
	}

	// 10-25x (medium)
	if maxWin >= 25 {
		boundaries = append(boundaries, 25)
	}

	// 25-50x (medium-large)
	if maxWin >= 50 {
		boundaries = append(boundaries, 50)
	}

	// 50-100x (large)
	if maxWin >= 100 {
		boundaries = append(boundaries, 100)
	}

	// 100-250x (very large)
	if maxWin >= 250 {
		boundaries = append(boundaries, 250)
	}

	// 250-500x (huge)
	if maxWin >= 500 {
		boundaries = append(boundaries, 500)
	}

	// 500-1000x (massive)
	if maxWin >= 1000 {
		boundaries = append(boundaries, 1000)
	}

	// 1000-2500x (epic)
	if maxWin >= 2500 {
		boundaries = append(boundaries, 2500)
	}

	// 2500-5000x (legendary)
	if maxWin >= 5000 {
		boundaries = append(boundaries, 5000)
	}

	// 5000+ (max)
	if maxWin > 5000 {
		boundaries = append(boundaries, maxWin)
	} else if len(boundaries) > 0 && boundaries[len(boundaries)-1] < maxWin {
		boundaries = append(boundaries, maxWin)
	}

	return boundaries
}

// getRTPDistribution returns RTP allocation percentages for each bucket
// Returns slice where sum = 100 (representing % of total RTP)
func (g *ConfigGenerator) getRTPDistribution(profile PlayerProfile, numBuckets int) []float64 {
	if numBuckets <= 0 {
		return []float64{}
	}

	// Base distributions for different bucket counts
	// Format: distribution for [sub1x, 1-2x, 2-5x, 5-10x, 10-25x, 25-50x, 50-100x, 100-250x, 250-500x, 500-1000x, 1000-2500x, 2500-5000x, 5000+]

	var baseDistribution []float64

	switch profile {
	case ProfileLowVol:
		// Heavy on small wins
		baseDistribution = []float64{35, 25, 15, 10, 7, 4, 2, 1, 0.5, 0.3, 0.1, 0.07, 0.03}

	case ProfileMediumVol:
		// Balanced distribution
		baseDistribution = []float64{20, 18, 15, 12, 10, 8, 6, 4, 3, 2, 1, 0.7, 0.3}

	case ProfileHighVol:
		// Heavy on big wins
		baseDistribution = []float64{10, 8, 8, 8, 10, 12, 14, 12, 8, 5, 3, 1.5, 0.5}

	default:
		// Default to medium volatility
		baseDistribution = []float64{20, 18, 15, 12, 10, 8, 6, 4, 3, 2, 1, 0.7, 0.3}
	}

	// Trim to actual number of buckets
	if numBuckets < len(baseDistribution) {
		baseDistribution = baseDistribution[:numBuckets]
	}

	// Normalize to 100%
	sum := 0.0
	for _, v := range baseDistribution {
		sum += v
	}

	result := make([]float64, len(baseDistribution))
	for i, v := range baseDistribution {
		result[i] = v / sum * 100
	}

	return result
}

// generateBuckets creates bucket configs from boundaries and RTP distribution
func (g *ConfigGenerator) generateBuckets(boundaries []float64, rtpDist []float64, targetRTP float64, profile PlayerProfile) []BucketConfig {
	numBuckets := len(boundaries) - 1
	if numBuckets <= 0 || len(rtpDist) != numBuckets {
		return []BucketConfig{}
	}

	buckets := make([]BucketConfig, numBuckets)

	for i := 0; i < numBuckets; i++ {
		minPayout := boundaries[i]
		maxPayout := boundaries[i+1]
		rtpPercent := rtpDist[i]

		// Calculate average payout for this bucket
		avgPayout := (minPayout + maxPayout) / 2
		if avgPayout <= 0 {
			avgPayout = maxPayout / 2
		}

		// Calculate frequency from RTP contribution
		// RTP contribution = (rtpPercent/100) * targetRTP
		// frequency = 1 / probability
		// probability = RTP_contribution / avgPayout

		rtpContribution := (rtpPercent / 100) * targetRTP
		probability := rtpContribution / avgPayout
		frequency := 1.0 / probability

		// Determine constraint type based on profile and bucket position
		bucket := BucketConfig{
			MinPayout: minPayout,
			MaxPayout: maxPayout,
		}

		// Use appropriate constraint type
		if g.shouldUseAuto(profile, i, numBuckets) {
			bucket.Type = ConstraintAuto
			bucket.AutoExponent = g.getExponent(profile)
		} else if frequency < 200 {
			// Low frequency = use frequency constraint
			bucket.Type = ConstraintFrequency
			bucket.Frequency = math.Round(frequency*10) / 10 // Round to 1 decimal
			if bucket.Frequency < 1 {
				bucket.Frequency = 1
			}
		} else {
			// High frequency = use RTP percent constraint
			bucket.Type = ConstraintRTPPercent
			bucket.RTPPercent = math.Round(rtpPercent*100) / 100 // Round to 2 decimals
			if bucket.RTPPercent < 0.01 {
				bucket.RTPPercent = 0.01
			}
		}

		buckets[i] = bucket
	}

	return buckets
}

// shouldUseAuto determines if a bucket should use AUTO constraint
func (g *ConfigGenerator) shouldUseAuto(profile PlayerProfile, bucketIdx, totalBuckets int) bool {
	// Last bucket is good for AUTO in high volatility
	if bucketIdx == totalBuckets-1 {
		return profile == ProfileHighVol
	}

	return false
}

// getExponent returns AUTO exponent for profile
func (g *ConfigGenerator) getExponent(profile PlayerProfile) float64 {
	switch profile {
	case ProfileLowVol:
		return 1.5 // Steeper = lower high payouts
	case ProfileHighVol:
		return 0.5 // Flatter = more high payouts
	default:
		return 1.0
	}
}

// toB64Config converts buckets to base64 encoded short config
func (g *ConfigGenerator) toB64Config(targetRTP float64, buckets []BucketConfig) string {
	shortBuckets := make([][]any, len(buckets))

	for i, b := range buckets {
		var typeInt int
		var value float64

		switch b.Type {
		case ConstraintFrequency:
			typeInt = 0
			value = b.Frequency
		case ConstraintRTPPercent:
			typeInt = 1
			value = b.RTPPercent
		case ConstraintAuto:
			typeInt = 2
			value = b.AutoExponent
		}

		shortBuckets[i] = []any{b.MinPayout, b.MaxPayout, typeInt, value}
	}

	short := ShortConfig{
		R: int(math.Round(targetRTP * 100)),
		B: shortBuckets,
	}

	jsonBytes, err := json.Marshal(short)
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(jsonBytes)
}

// calculateStats computes statistics for the config
func (g *ConfigGenerator) calculateStats(buckets []BucketConfig, rtpDist []float64, targetRTP float64) ConfigStats {
	rtpDistribution := make(map[string]float64)
	var totalWinProb float64
	var maxWinFreq float64

	for i, b := range buckets {
		rangeKey := fmt.Sprintf("%.0f-%.0f", b.MinPayout, b.MaxPayout)
		if i < len(rtpDist) {
			rtpDistribution[rangeKey] = math.Round(rtpDist[i]*100) / 100
		}

		// Calculate win probability for this bucket
		avgPayout := (b.MinPayout + b.MaxPayout) / 2
		if avgPayout <= 0 {
			avgPayout = b.MaxPayout / 2
		}

		var prob float64
		switch b.Type {
		case ConstraintFrequency:
			prob = 1.0 / b.Frequency
		case ConstraintRTPPercent:
			rtpContrib := (b.RTPPercent / 100) * targetRTP
			prob = rtpContrib / avgPayout
		case ConstraintAuto:
			// Estimate for AUTO - will be calculated properly during optimization
			rtpEstimate := 0.05 * targetRTP // Assume 5% of RTP
			prob = rtpEstimate / avgPayout
		}

		totalWinProb += prob

		// Track max win frequency
		if i == len(buckets)-1 {
			maxWinFreq = 1.0 / prob
		}
	}

	avgHitRate := 1.0 / totalWinProb
	if math.IsInf(avgHitRate, 1) || math.IsNaN(avgHitRate) {
		avgHitRate = 0
	}

	return ConfigStats{
		TotalBuckets:    len(buckets),
		RTPDistribution: rtpDistribution,
		AvgHitRate:      math.Round(avgHitRate*10) / 10,
		MaxWinFreq:      math.Round(maxWinFreq),
	}
}

// getProfileName returns human-readable profile name
func (g *ConfigGenerator) getProfileName(profile PlayerProfile) string {
	names := map[PlayerProfile]string{
		ProfileLowVol:    "Low Volatility",
		ProfileMediumVol: "Medium Volatility",
		ProfileHighVol:   "High Volatility",
	}

	if name, ok := names[profile]; ok {
		return name
	}
	return string(profile)
}

// ValidateGeneratedConfig validates a generated config is mathematically sound
func ValidateGeneratedConfig(config *GeneratedConfig) error {
	if config.TargetRTP <= 0 || config.TargetRTP > 1 {
		return fmt.Errorf("invalid target RTP: %.4f", config.TargetRTP)
	}

	if config.MaxWin <= 0 {
		return fmt.Errorf("invalid max win: %.2f", config.MaxWin)
	}

	if len(config.Buckets) == 0 {
		return fmt.Errorf("no buckets generated")
	}

	// Calculate total RTP contribution to ensure it's valid
	var totalRTPContribution float64

	for _, b := range config.Buckets {
		avgPayout := (b.MinPayout + b.MaxPayout) / 2
		if avgPayout <= 0 {
			avgPayout = b.MaxPayout / 2
		}

		var contribution float64
		switch b.Type {
		case ConstraintFrequency:
			prob := 1.0 / b.Frequency
			contribution = prob * avgPayout
		case ConstraintRTPPercent:
			contribution = (b.RTPPercent / 100) * config.TargetRTP
		case ConstraintAuto:
			// AUTO buckets use remaining RTP
			continue
		}

		totalRTPContribution += contribution
	}

	// Total contribution should not exceed target (AUTO handles remainder)
	if totalRTPContribution > config.TargetRTP*1.1 { // 10% tolerance
		return fmt.Errorf("RTP overcommitment: %.4f > %.4f", totalRTPContribution, config.TargetRTP)
	}

	return nil
}
