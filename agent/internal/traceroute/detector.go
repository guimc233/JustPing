package traceroute

import (
	"math"
	"sync"
)

// LatencyShiftDetector monitors individual ping RTTs for a single target and detects
// persistent step-change latency shifts (as opposed to temporary jitter/spikes).
type LatencyShiftDetector struct {
	mu             sync.Mutex
	history        []float64
	maxHistory     int
	baseline       float64
	pendingShift   float64
	streak         int
	requiredStreak int
}

func NewLatencyShiftDetector(requiredStreak int) *LatencyShiftDetector {
	if requiredStreak <= 0 {
		requiredStreak = 3
	}
	return &LatencyShiftDetector{
		history:        make([]float64, 0, 10),
		maxHistory:     10,
		requiredStreak: requiredStreak,
	}
}

// Feed receives a new successful ping RTT measurement.
// Returns true if a persistent latency shift is confirmed.
func (d *LatencyShiftDetector) Feed(rtt float64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if rtt <= 0 {
		return false
	}

	// Baseline initialization phase (require at least 5 baseline samples)
	if len(d.history) < 5 {
		d.history = append(d.history, rtt)
		d.baseline = d.calcAvg()
		return false
	}

	diff := math.Abs(rtt - d.baseline)
	// Significant shift condition: absolute difference >= 25ms OR (>= 15ms and >= 30% relative change)
	isStep := diff >= 25.0 || (diff >= 15.0 && d.baseline > 0 && (diff/d.baseline) >= 0.30)

	if !isStep {
		// New RTT is within the normal baseline range
		if d.streak > 0 {
			// Dropped back to baseline; previous high was just a transient spike
			d.streak = 0
		}
		d.addHistory(rtt)
		d.baseline = d.calcAvg()
		return false
	}

	// The RTT is significantly different from baseline
	if d.streak == 0 {
		// Initial shift observed; start monitoring persistence
		d.pendingShift = rtt
		d.streak = 1
		return false
	}

	// Verify if the new RTT continues to stay in the shifted band
	shiftDiff := math.Abs(rtt - d.pendingShift)
	consistentWithShift := shiftDiff <= 20.0 || (d.pendingShift > 0 && (shiftDiff/d.pendingShift) <= 0.25)

	if consistentWithShift {
		d.streak++
		if d.streak >= d.requiredStreak {
			// Persistent shift confirmed!
			// Update baseline to the new stable latency level
			d.history = []float64{rtt}
			d.baseline = rtt
			d.streak = 0
			return true
		}
		return false
	}

	// New RTT diverges from both baseline and pending shift; reset monitoring
	d.pendingShift = rtt
	d.streak = 1
	return false
}

func (d *LatencyShiftDetector) calcAvg() float64 {
	if len(d.history) == 0 {
		return 0
	}
	var sum float64
	for _, v := range d.history {
		sum += v
	}
	return sum / float64(len(d.history))
}

func (d *LatencyShiftDetector) addHistory(val float64) {
	d.history = append(d.history, val)
	if len(d.history) > d.maxHistory {
		d.history = d.history[len(d.history)-d.maxHistory:]
	}
}
