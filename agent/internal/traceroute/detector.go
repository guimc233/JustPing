package traceroute

import (
	"math"
	"sort"
	"sync"
)

// LatencyShiftDetector monitors ping RTTs for a single target and detects
// true persistent routing shifts rather than cross-border / peak-hour congestion jitter.
//
// In cross-border / GFW environments:
// - Evening peak hours cause queuing, packet drops, and high jitter (+30ms ~ +100ms fluctuates up and down wildly).
// - Normal congestion exhibits high standard deviation (jitter) across samples.
// - A true route shift (e.g. 163 -> CN2 GIA, or Hong Kong -> US reroute) results in:
//   1. A significant change in the baseline latency (minimum or stable median shifts by >= 40ms or >= 45%).
//   2. The new state becomes stable at the new plateau (low internal variance), rather than erratic oscillating jitter.
//   3. The shift persists for a substantial duration (e.g. 30 consecutive 30s samples = 15 minutes).
type LatencyShiftDetector struct {
	mu             sync.Mutex
	history        []float64
	maxHistory     int
	baseline       float64
	shiftSamples   []float64
	requiredStreak int
}

func NewLatencyShiftDetector(requiredStreak int) *LatencyShiftDetector {
	if requiredStreak <= 0 {
		requiredStreak = 30 // 30 consecutive 30s samples = 15 minutes of sustained plateau
	}
	return &LatencyShiftDetector{
		history:        make([]float64, 0, 30),
		maxHistory:     30,
		requiredStreak: requiredStreak,
		shiftSamples:   make([]float64, 0, requiredStreak),
	}
}

// Feed receives a new successful ping RTT measurement.
// Returns true only when a genuine, persistent route-level plateau shift is confirmed.
func (d *LatencyShiftDetector) Feed(rtt float64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if rtt <= 0 {
		return false
	}

	// 1. Warm-up baseline with at least 8 samples (~4 minutes)
	if len(d.history) < 8 {
		d.history = append(d.history, rtt)
		d.baseline = d.calcMedian(d.history)
		return false
	}

	// Current baseline median and noise floor
	baseMedian := d.calcMedian(d.history)
	d.baseline = baseMedian
	baseJitter := d.calcMAD(d.history, baseMedian) // Median Absolute Deviation

	diff := math.Abs(rtt - baseMedian)

	// Threshold for candidate route shift:
	// - Must exceed 3x normal baseline jitter to filter out existing congestion
	// - Must be >= 40ms absolute shift OR (>= 25ms and >= 45% of baseline)
	minDiffRequired := math.Max(40.0, baseJitter*3.0)
	isCandidateShift := diff >= minDiffRequired || (diff >= 25.0 && baseMedian > 0 && (diff/baseMedian) >= 0.45 && diff >= baseJitter*2.5)

	if !isCandidateShift {
		// Sample is within normal baseline range
		// If we were tracking a candidate shift, this return to normal baseline proves it was just congestion/jitter!
		if len(d.shiftSamples) > 0 {
			d.shiftSamples = d.shiftSamples[:0]
		}
		d.addHistory(rtt)
		return false
	}

	// 2. Candidate shift observed; collect consecutive shifted samples
	d.shiftSamples = append(d.shiftSamples, rtt)

	if len(d.shiftSamples) < d.requiredStreak {
		return false
	}

	// 3. We reached requiredStreak samples. Verify if shiftSamples form a STABLE new plateau:
	// A real route change produces a relatively uniform new latency band.
	// If the candidate samples themselves oscillate wildly (e.g. 50ms, 160ms, 80ms, 200ms),
	// it is peak-hour congestion/packet retransmissions, NOT a route change.
	newMedian := d.calcMedian(d.shiftSamples)
	newMAD := d.calcMAD(d.shiftSamples, newMedian)

	// Shift distance between old baseline median and new plateau median
	shiftDist := math.Abs(newMedian - baseMedian)
	if shiftDist < 30.0 && (baseMedian == 0 || (shiftDist/baseMedian) < 0.40) {
		// New median is too close to old baseline (spurious)
		d.shiftSamples = d.shiftSamples[:0]
		return false
	}

	// New plateau stability test:
	// The median absolute deviation in the new window must be reasonably tight (<= 25ms or <= 25% of new median)
	isStableNewRoute := newMAD <= 25.0 || (newMedian > 0 && (newMAD/newMedian) <= 0.25)

	if !isStableNewRoute {
		// High internal oscillation -> evening jitter / packet bufferbloat, ignore!
		// Drop oldest sample to keep sliding
		d.shiftSamples = d.shiftSamples[1:]
		return false
	}

	// 4. Confirmed persistent route plateau shift!
	// Reset shift tracker, re-initialize history to the new plateau
	d.history = make([]float64, len(d.shiftSamples))
	copy(d.history, d.shiftSamples)
	d.baseline = newMedian
	d.shiftSamples = d.shiftSamples[:0]

	return true
}

func (d *LatencyShiftDetector) addHistory(val float64) {
	d.history = append(d.history, val)
	if len(d.history) > d.maxHistory {
		d.history = d.history[len(d.history)-d.maxHistory:]
	}
}

// calcMedian calculates the median of a slice of float64
func (d *LatencyShiftDetector) calcMedian(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	copied := make([]float64, len(vals))
	copy(copied, vals)
	sort.Float64s(copied)
	mid := len(copied) / 2
	if len(copied)%2 == 1 {
		return copied[mid]
	}
	return (copied[mid-1] + copied[mid]) / 2.0
}

// calcMAD calculates Median Absolute Deviation around center: median(|x - center|)
func (d *LatencyShiftDetector) calcMAD(vals []float64, center float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	devs := make([]float64, len(vals))
	for i, v := range vals {
		devs[i] = math.Abs(v - center)
	}
	return d.calcMedian(devs)
}
