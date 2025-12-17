package optimizer

// Helper functions for weight optimization

// calculateRTPFromWeights calculates RTP directly from weights
func calculateRTPFromWeights(weights []uint64, payouts []float64) float64 {
	total := sumUint64(weights)
	if total == 0 {
		return 0
	}

	rtp := 0.0
	for i, w := range weights {
		prob := float64(w) / float64(total)
		rtp += payouts[i] * prob
	}
	return rtp
}

// sumUint64 sums uint64 slice
func sumUint64(values []uint64) uint64 {
	var total uint64
	for _, v := range values {
		total += v
	}
	return total
}
