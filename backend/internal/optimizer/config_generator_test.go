package optimizer

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestConfigGenerator_GenerateAllProfiles(t *testing.T) {
	gen := NewConfigGenerator()

	// Test with typical parameters
	targetRTP := 0.96
	maxWin := 5000.0

	response := gen.GenerateAllProfiles(targetRTP, maxWin)

	if len(response.Configs) != 3 {
		t.Errorf("Expected 3 profiles, got %d", len(response.Configs))
	}

	// Verify each profile
	expectedProfiles := []PlayerProfile{
		ProfileLowVol,
		ProfileMediumVol,
		ProfileHighVol,
	}

	for i, expected := range expectedProfiles {
		config := response.Configs[i]
		if config.Profile != expected {
			t.Errorf("Config %d: expected profile %s, got %s", i, expected, config.Profile)
		}

		// Verify config has buckets
		if len(config.Buckets) == 0 {
			t.Errorf("Config %d (%s): no buckets generated", i, config.Profile)
		}

		// Verify b64 config is valid
		if config.B64Config == "" {
			t.Errorf("Config %d (%s): no b64 config", i, config.Profile)
		}

		// Decode and verify b64
		decoded, err := base64.StdEncoding.DecodeString(config.B64Config)
		if err != nil {
			t.Errorf("Config %d (%s): failed to decode b64: %v", i, config.Profile, err)
			continue
		}

		var short ShortConfig
		if err := json.Unmarshal(decoded, &short); err != nil {
			t.Errorf("Config %d (%s): failed to parse short config: %v", i, config.Profile, err)
			continue
		}

		// Verify RTP matches
		if short.R != int(targetRTP*100) {
			t.Errorf("Config %d (%s): expected RTP %d, got %d", i, config.Profile, int(targetRTP*100), short.R)
		}

		// Verify bucket count matches
		if len(short.B) != len(config.Buckets) {
			t.Errorf("Config %d (%s): bucket count mismatch: %d vs %d", i, config.Profile, len(short.B), len(config.Buckets))
		}

		t.Logf("Profile %s: %d buckets, avg hit 1:%.1f, max win freq 1:%.0f",
			config.ProfileName, config.Stats.TotalBuckets, config.Stats.AvgHitRate, config.Stats.MaxWinFreq)
	}
}

func TestConfigGenerator_GenerateConfig_LowVol(t *testing.T) {
	gen := NewConfigGenerator()
	config := gen.GenerateConfig(0.96, 5000, ProfileLowVol)

	// Low vol should have higher avg hit rate (more frequent wins)
	if config.Stats.AvgHitRate == 0 {
		t.Error("Low vol should have non-zero avg hit rate")
	}

	// Validate the config
	if err := ValidateGeneratedConfig(config); err != nil {
		t.Errorf("Generated config is invalid: %v", err)
	}

	t.Logf("Low vol: %d buckets, hit rate 1:%.1f", config.Stats.TotalBuckets, config.Stats.AvgHitRate)
	for _, b := range config.Buckets {
		t.Logf("  %.0f-%.0fx: %s", b.MinPayout, b.MaxPayout, b.Type)
	}
}

func TestConfigGenerator_GenerateConfig_HighVol(t *testing.T) {
	gen := NewConfigGenerator()
	config := gen.GenerateConfig(0.96, 5000, ProfileHighVol)

	// High vol should have lower avg hit rate (less frequent wins)
	if config.Stats.AvgHitRate == 0 {
		t.Error("High vol should have non-zero avg hit rate")
	}

	// Validate the config
	if err := ValidateGeneratedConfig(config); err != nil {
		t.Errorf("Generated config is invalid: %v", err)
	}

	t.Logf("High vol: %d buckets, hit rate 1:%.1f", config.Stats.TotalBuckets, config.Stats.AvgHitRate)
	for _, b := range config.Buckets {
		t.Logf("  %.0f-%.0fx: %s", b.MinPayout, b.MaxPayout, b.Type)
	}
}

func TestConfigGenerator_DifferentMaxWins(t *testing.T) {
	gen := NewConfigGenerator()

	testCases := []struct {
		maxWin         float64
		expectedMinBuckets int
	}{
		{100, 5},     // Small max win
		{1000, 8},    // Medium max win
		{5000, 10},   // Large max win
		{10000, 12},  // Very large max win
	}

	for _, tc := range testCases {
		config := gen.GenerateConfig(0.96, tc.maxWin, ProfileMediumVol)

		if len(config.Buckets) < tc.expectedMinBuckets {
			t.Errorf("Max win %.0f: expected at least %d buckets, got %d",
				tc.maxWin, tc.expectedMinBuckets, len(config.Buckets))
		}

		// Verify last bucket covers max win
		lastBucket := config.Buckets[len(config.Buckets)-1]
		if lastBucket.MaxPayout < tc.maxWin {
			t.Errorf("Max win %.0f: last bucket max %.0f doesn't cover max win",
				tc.maxWin, lastBucket.MaxPayout)
		}

		t.Logf("Max win %.0f: %d buckets, last bucket %.0f-%.0f",
			tc.maxWin, len(config.Buckets), lastBucket.MinPayout, lastBucket.MaxPayout)
	}
}

func TestConfigGenerator_B64RoundTrip(t *testing.T) {
	gen := NewConfigGenerator()
	config := gen.GenerateConfig(0.97, 3000, ProfileMediumVol)

	// Decode b64
	decoded, err := base64.StdEncoding.DecodeString(config.B64Config)
	if err != nil {
		t.Fatalf("Failed to decode b64: %v", err)
	}

	var short ShortConfig
	if err := json.Unmarshal(decoded, &short); err != nil {
		t.Fatalf("Failed to parse short config: %v", err)
	}

	// Verify RTP
	if short.R != 97 {
		t.Errorf("Expected RTP 97, got %d", short.R)
	}

	// Verify buckets match
	if len(short.B) != len(config.Buckets) {
		t.Errorf("Bucket count mismatch: %d vs %d", len(short.B), len(config.Buckets))
	}

	for i, b := range short.B {
		if len(b) != 4 {
			t.Errorf("Bucket %d: expected 4 elements, got %d", i, len(b))
			continue
		}

		minPayout := b[0].(float64)
		maxPayout := b[1].(float64)
		typeInt := int(b[2].(float64))

		expected := config.Buckets[i]
		if minPayout != expected.MinPayout {
			t.Errorf("Bucket %d: min_payout mismatch: %.2f vs %.2f", i, minPayout, expected.MinPayout)
		}
		if maxPayout != expected.MaxPayout {
			t.Errorf("Bucket %d: max_payout mismatch: %.2f vs %.2f", i, maxPayout, expected.MaxPayout)
		}

		// Verify type
		expectedType := 0
		switch expected.Type {
		case ConstraintFrequency:
			expectedType = 0
		case ConstraintRTPPercent:
			expectedType = 1
		case ConstraintAuto:
			expectedType = 2
		}
		if typeInt != expectedType {
			t.Errorf("Bucket %d: type mismatch: %d vs %d", i, typeInt, expectedType)
		}
	}

	t.Logf("B64 config: %s", config.B64Config)
}

func TestConfigGenerator_Validation(t *testing.T) {
	gen := NewConfigGenerator()

	// All profiles should produce valid configs
	profiles := []PlayerProfile{
		ProfileLowVol,
		ProfileMediumVol,
		ProfileHighVol,
	}

	for _, profile := range profiles {
		config := gen.GenerateConfig(0.96, 5000, profile)

		if err := ValidateGeneratedConfig(config); err != nil {
			t.Errorf("Profile %s: validation failed: %v", profile, err)
		}
	}
}
