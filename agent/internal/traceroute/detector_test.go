package traceroute

import "testing"

func TestLatencyShiftDetector(t *testing.T) {
	detector := NewLatencyShiftDetector(3)

	// Feed 5 baseline samples around 30ms
	for i := 0; i < 5; i++ {
		if detector.Feed(30.0) {
			t.Errorf("expected no shift during baseline initialization")
		}
	}

	// Single transient spike (e.g. 120ms) followed by return to baseline
	if detector.Feed(120.0) {
		t.Errorf("single spike should not trigger shift")
	}
	if detector.Feed(31.0) {
		t.Errorf("return to baseline should cancel shift streak")
	}
	if detector.streak != 0 {
		t.Errorf("expected streak reset to 0, got %d", detector.streak)
	}

	// Persistent shift from 30ms -> 85ms (sustained for 3 probes)
	if detector.Feed(85.0) {
		t.Errorf("streak 1 should not trigger")
	}
	if detector.Feed(87.0) {
		t.Errorf("streak 2 should not trigger")
	}
	if !detector.Feed(84.0) {
		t.Errorf("streak 3 should confirm persistent shift!")
	}
}
