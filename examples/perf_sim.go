package examples

import (
	"math/rand"
	"strings"
	"time"
)

// TierMultiplier returns a latency multiplier for the given user tier.
// premium=1× (fastest), standard=2×, free=5× (slowest).
// Unknown tiers default to standard.
func TierMultiplier(tier string) float64 {
	switch strings.ToLower(tier) {
	case "premium":
		return 1.0
	case "standard":
		return 2.0
	case "free":
		return 5.0
	default:
		return 2.0
	}
}

// PayloadLatency returns a base latency driven by payload size.
// Adds 1ms per 100 bytes on top of a 5ms floor.
func PayloadLatency(sizeBytes int) time.Duration {
	base := 5 * time.Millisecond
	extra := time.Duration(sizeBytes/100) * time.Millisecond
	return base + extra
}

// SpikeLatency returns either the base latency or a spiked value.
// spikeRate is the probability [0,1] of a spike occurring.
// spikeMultiplier is the factor applied to base on a spike.
func SpikeLatency(base time.Duration, spikeRate, spikeMultiplier float64) time.Duration {
	if spikeRate > 0 && rand.Float64() < spikeRate {
		return time.Duration(float64(base) * spikeMultiplier)
	}
	return base
}

// SimulateLatency combines tier, spike, and base latency into a single sleep
// duration. spikeRate=0 disables spikes; spikeMultiplier=1 means no amplification.
func SimulateLatency(base time.Duration, tier string, spikeRate, spikeMultiplier float64) time.Duration {
	tiered := time.Duration(float64(base) * TierMultiplier(tier))
	d := SpikeLatency(tiered, spikeRate, spikeMultiplier)
	if d < 0 {
		d = 0
	}
	return d
}

// Sleep calls SimulateLatency and blocks for the resulting duration.
func Sleep(base time.Duration, tier string, spikeRate, spikeMultiplier float64) {
	time.Sleep(SimulateLatency(base, tier, spikeRate, spikeMultiplier))
}
