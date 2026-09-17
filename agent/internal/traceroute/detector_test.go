package traceroute

import "testing"

func TestLatencyShiftDetector(t *testing.T) {
	detector := NewLatencyShiftDetector(30)

	// Feed 8 baseline samples around 35ms
	baselineSamples := []float64{34, 35, 36, 35, 34, 35, 36, 35}
	for _, s := range baselineSamples {
		if detector.Feed(s) {
			t.Errorf("expected no shift during baseline initialization")
		}
	}

	// Case 1: Evening peak-hour erratic jitter (jumping up and down: 40, 110, 50, 140, 45, 120)
	// Must NOT trigger traceroute!
	jitterSamples := []float64{110, 45, 130, 38, 120, 50, 140, 42}
	for _, s := range jitterSamples {
		if detector.Feed(s) {
			t.Errorf("peak-hour oscillation (%.1fms) should NOT trigger route shift!", s)
		}
	}

	// Case 2: High latency wild oscillation (e.g. 100, 180, 70, 190, 80, 210)
	// Internal MAD is high, must NOT trigger route shift!
	wildSamples := []float64{100, 180, 70, 190, 80, 210}
	for _, s := range wildSamples {
		if detector.Feed(s) {
			t.Errorf("unstable high-jitter sample (%.1fms) should NOT trigger route shift!", s)
		}
	}

	// Case 3: Genuine stable route change (e.g. rerouted from 35ms -> 160ms stable plateau)
	// Sustained for 30 consecutive 30s samples (15 minutes)
	var shiftTriggered bool
	for i := 0; i < 30; i++ {
		val := 160.0 + float64(i%3)
		if detector.Feed(val) {
			shiftTriggered = true
			break
		}
	}

	if !shiftTriggered {
		t.Errorf("expected genuine stable route shift to 160ms sustained for 15 minutes to trigger!")
	}
}
