package examples

import (
	"testing"
	"time"
)

func TestTierMultiplier(t *testing.T) {
	tests := []struct {
		tier string
		want float64
	}{
		{"premium", 1.0},
		{"standard", 2.0},
		{"free", 5.0},
		{"unknown", 2.0}, // default to standard
		{"", 2.0},
	}

	for _, tt := range tests {
		got := TierMultiplier(tt.tier)
		if got != tt.want {
			t.Errorf("TierMultiplier(%q) = %.1f, want %.1f", tt.tier, got, tt.want)
		}
	}
}

func TestPayloadLatency(t *testing.T) {
	// Base latency should increase with payload size
	small := PayloadLatency(100)
	medium := PayloadLatency(1000)
	large := PayloadLatency(10000)

	if small >= medium {
		t.Errorf("small (%v) should be less than medium (%v)", small, medium)
	}
	if medium >= large {
		t.Errorf("medium (%v) should be less than large (%v)", medium, large)
	}
	// Empty payload returns base latency
	base := PayloadLatency(0)
	if base <= 0 {
		t.Error("base latency should be > 0")
	}
}

func TestSpikeLatency(t *testing.T) {
	base := 10 * time.Millisecond
	spikeMultiplier := 10.0

	// Run enough iterations that we statistically see at least one spike
	sawSpike := false
	sawNormal := false

	for i := 0; i < 200; i++ {
		got := SpikeLatency(base, 0.1, spikeMultiplier)
		if got > base {
			sawSpike = true
		}
		if got == base {
			sawNormal = true
		}
		if sawSpike && sawNormal {
			break
		}
	}

	if !sawSpike {
		t.Error("expected at least one spike in 200 iterations at 10% rate")
	}
	if !sawNormal {
		t.Error("expected at least one normal latency in 200 iterations")
	}
}

func TestSpikeLatency_ZeroRate(t *testing.T) {
	base := 5 * time.Millisecond
	for i := 0; i < 50; i++ {
		got := SpikeLatency(base, 0.0, 10.0)
		if got != base {
			t.Errorf("zero spike rate should always return base, got %v", got)
		}
	}
}

func TestSpikeLatency_AlwaysSpike(t *testing.T) {
	base := 5 * time.Millisecond
	multiplier := 3.0
	expected := time.Duration(float64(base) * multiplier)

	for i := 0; i < 10; i++ {
		got := SpikeLatency(base, 1.0, multiplier)
		if got != expected {
			t.Errorf("100%% spike rate should always return %v, got %v", expected, got)
		}
	}
}

func TestSimulateLatency_AppliesTierAndSpike(t *testing.T) {
	base := 5 * time.Millisecond

	premiumLatency := SimulateLatency(base, "premium", 0.0, 1.0)
	freeLatency := SimulateLatency(base, "free", 0.0, 1.0)

	// free tier should take longer than premium
	if freeLatency <= premiumLatency {
		t.Errorf("free (%v) should be slower than premium (%v)", freeLatency, premiumLatency)
	}
}

func TestSimulateLatency_NegativeDurationClamped(t *testing.T) {
	// Should never return negative duration
	d := SimulateLatency(0, "premium", 0.0, 1.0)
	if d < 0 {
		t.Errorf("latency should never be negative, got %v", d)
	}
}

func TestTierMultiplier_CaseInsensitive(t *testing.T) {
	// Tier matching should handle mixed case
	tests := []struct {
		tier string
		want float64
	}{
		{"Premium", 1.0},
		{"STANDARD", 2.0},
		{"Free", 5.0},
	}
	for _, tt := range tests {
		got := TierMultiplier(tt.tier)
		if got != tt.want {
			t.Errorf("TierMultiplier(%q) = %.1f, want %.1f", tt.tier, got, tt.want)
		}
	}
}
