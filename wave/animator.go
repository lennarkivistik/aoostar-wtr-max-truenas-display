// Package wave provides continuous flowing wave animation for sparklines.
// The wave continuously scrolls left with new data appearing on the right,
// creating a smooth, always-moving visualization.
package wave

import (
	"go-wtr-display/display"
	"sync"
	"time"
)

// Sample represents a single data point at a specific time.
type Sample struct {
	Rx        float64
	Tx        float64
	Timestamp time.Time
}

// Animator manages a continuously flowing wave using time-based positioning.
// This ensures zero "jumping" - data points stay fixed in time and just slide off-screen.
type Animator struct {
	mu sync.Mutex

	// Configuration
	ScrollSpeed float64 // Pixels per second to scroll left
	WaveWidth   int     // Total visual width of the wave

	// Data buffer (dynamic slice)
	// We keep samples until they are well off-screen
	samples []Sample

	// Animation state
	startTime   time.Time // When animation started
	smoothedMax float64   // Smoothly interpolated max value for scaling
}

// NewAnimator creates a new continuous wave animator.
// bufferSize is ignored in this new implementation as we manage history dynamically.
func NewAnimator(bufferSize int) *Animator {
	return &Animator{
		// Zoomed-in view: slow scroll, short history = BIG beautiful waves
		// At 120px/sec with 960px screen, ~8 seconds visible
		// With samples every 1s, that's ~8 data points spread across the screen
		ScrollSpeed: 120, // Pixels per second (moderate scroll)
		WaveWidth:   960, // Match screen width - no extra history
		samples:     make([]Sample, 0, 20),
		startTime:   time.Now(),
	}
}

// AddSample adds a new data point.
func (a *Animator) AddSample(rx, tx float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.samples = append(a.samples, Sample{
		Rx:        rx,
		Tx:        tx,
		Timestamp: time.Now(),
	})

	// Prune old samples that are too far off-screen to matter
	// We keep enough history to cover the WaveWidth plus some buffer
	a.pruneOldSamples()
}

// pruneOldSamples removes samples that have scrolled off-screen left.
func (a *Animator) pruneOldSamples() {
	if len(a.samples) < 3 {
		return
	}

	now := time.Now()
	// Keep samples for ~1.5x the visible duration (some buffer for smooth entry/exit)
	visibleDuration := time.Duration(float64(a.WaveWidth)/a.ScrollSpeed*1.5) * time.Second
	cutoffTime := now.Add(-visibleDuration)

	// Find first sample that's still relevant
	cutIdx := 0
	for i, s := range a.samples {
		if s.Timestamp.After(cutoffTime) {
			cutIdx = i
			break
		}
	}

	// Keep one extra sample before cutoff for smooth curve anchoring
	if cutIdx > 1 {
		cutIdx--
	}

	// Remove old samples
	if cutIdx > 0 {
		a.samples = a.samples[cutIdx:]
	}
}

// GetWavePoints returns visual points for rendering the wave.
// Returns slice of XY points for Rx and Tx lines, mapped to screen coordinates.
// Auto-scales to visible data so small variations fill the wave height.
// Includes off-screen points for smooth edge rendering.
// OPTIMIZED: Avoids intermediate allocations by doing two passes over samples.
func (a *Animator) GetWavePoints(screenHeight int) ([]display.WavePoint, []display.WavePoint) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.samples) < 2 {
		return nil, nil
	}

	now := time.Now()
	screenWidth := 960       // Screen width for visibility check
	offScreenBuffer := 100.0 // Buffer outside frame for smooth curve entry/exit

	// First pass: find visible range and max values
	// We only care about samples that will end up on screen (or in buffer)
	visibleMaxRx := 0.0
	visibleMaxTx := 0.0

	startIndex := -1
	count := 0

	for i, s := range a.samples {
		age := now.Sub(s.Timestamp).Seconds()
		x := float64(screenWidth) + offScreenBuffer - (age * a.ScrollSpeed)

		// Check visibility
		if x >= -offScreenBuffer && x <= float64(screenWidth)+offScreenBuffer {
			if startIndex == -1 {
				startIndex = i
			}
			count++

			if s.Rx > visibleMaxRx {
				visibleMaxRx = s.Rx
			}
			if s.Tx > visibleMaxTx {
				visibleMaxTx = s.Tx
			}
		} else if startIndex != -1 && x < -offScreenBuffer {
			// Optimization: Since samples are ordered by time (newest last),
			// and x decreases as age increases (older samples move left),
			// once we hit a sample that is too far LEFT (x < -buffer),
			// all previous samples (which are even older) are also too far left.
			// However, our loop is oldest->newest or newest->oldest?
			// append adds to end. Range iterates 0..N.
			// 0 is oldest. N is newest.
			// Oldest sample has largest age -> smallest X.
			// Newest sample has smallest age -> largest X.
			// So we start with small X (left) and go to large X (right).

			// If we haven't found start yet (startIndex == -1), it means this sample
			// is still off-screen to the LEFT. We keep going.
			// If we HAVE found start, and now x > rightBuffer? No.
		}
	}

	// Because we iterate oldest -> newest:
	// x goes from Negative (left) -> Positive (right)
	// We want range where x is in [-buffer, width+buffer]

	// Let's simpler logic: just indices
	// We can allocate exact size now
	if count == 0 {
		return nil, nil
	}

	// Use the max of visible Rx and Tx for consistent scaling
	targetMax := visibleMaxRx
	if visibleMaxTx > targetMax {
		targetMax = visibleMaxTx
	}

	// Minimum threshold so flat/zero lines still show some movement
	minThreshold := 1000.0 // 1 KB/s minimum scale
	if targetMax < minThreshold {
		targetMax = minThreshold
	}

	// Add headroom so peaks don't hit the top
	targetMax *= 1.15

	// Smooth the max value transition
	if a.smoothedMax == 0 {
		a.smoothedMax = targetMax
	} else if targetMax > a.smoothedMax {
		// Scale UP quickly (0.3 = fast response to peaks)
		a.smoothedMax += (targetMax - a.smoothedMax) * 0.3
	} else {
		// Scale DOWN
		// Increased decay from 0.05 to 0.1 for faster recovery after peaks
		// At 15 FPS, this drops to ~20% of previous max in 1 second
		a.smoothedMax += (targetMax - a.smoothedMax) * 0.1
	}

	maxVal := a.smoothedMax

	// Second pass: generate points directly into result slices
	rxPoints := make([]display.WavePoint, 0, count)
	txPoints := make([]display.WavePoint, 0, count)

	// Optimization: iterate only the relevant range if we tracked it correctly
	// But simple iteration is fast enough for <20 items
	for _, s := range a.samples {
		age := now.Sub(s.Timestamp).Seconds()
		x := float64(screenWidth) + offScreenBuffer - (age * a.ScrollSpeed)

		if x >= -offScreenBuffer && x <= float64(screenWidth)+offScreenBuffer {
			rxY := s.Rx / maxVal
			txY := s.Tx / maxVal

			// Clamp to 0-1 range
			if rxY > 1.0 {
				rxY = 1.0
			}
			if txY > 1.0 {
				txY = 1.0
			}

			rxPoints = append(rxPoints, display.WavePoint{X: x, Y: rxY})
			txPoints = append(txPoints, display.WavePoint{X: x, Y: txY})
		}
	}

	return rxPoints, txPoints
}

// IsAnimating always returns true for continuous animation.
func (a *Animator) IsAnimating() bool {
	return true
}

// HasData returns true if we have enough samples to render.
func (a *Animator) HasData() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.samples) >= 2
}

// GetConfig returns wave width for rendering calculations.
func (a *Animator) GetConfig() (waveWidth int, scrollSpeed float64) {
	return a.WaveWidth, a.ScrollSpeed
}
