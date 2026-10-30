// Package wave maps bounded, timestamped throughput history onto the display.
package wave

import (
	"math"
	"sync"
	"time"

	"go-wtr-display/display"
)

type Sample struct {
	Rx, Tx    float64
	Timestamp time.Time
}

type Animator struct {
	mu          sync.Mutex
	history     time.Duration
	samples     []Sample
	rx, tx      []display.WavePoint
	smoothedMax float64
	lastScale   time.Time
	fixedScale  bool
}

// NewAnimator retains one sample per second plus interpolation anchors.
func NewAnimator(seconds int) *Animator {
	return NewAnimatorWithHistory(time.Duration(seconds) * time.Second)
}

func NewAnimatorWithHistory(history time.Duration) *Animator {
	if history < 5*time.Second {
		history = 5 * time.Second
	}
	if history > 300*time.Second {
		history = 300 * time.Second
	}
	seconds := int(math.Ceil(history.Seconds()))
	if seconds < 5 {
		seconds = 5
	}
	if seconds > 300 {
		seconds = 300
	}
	return &Animator{history: history,
		samples: make([]Sample, 0, seconds+4),
		rx:      make([]display.WavePoint, 0, seconds+5), tx: make([]display.WavePoint, 0, seconds+5)}
}

func (a *Animator) AddSample(rx, tx float64) { a.AddSampleAt(rx, tx, time.Now()) }

func (a *Animator) AddSampleAt(rx, tx float64, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n := len(a.samples); n > 0 {
		gap := now.Sub(a.samples[n-1].Timestamp)
		if gap <= 0 {
			return
		}
		// Do not draw a fictitious bridge over an outage or night-mode pause.
		if gap > 3*time.Second {
			a.samples = a.samples[:0]
			a.smoothedMax = 0
			a.lastScale = time.Time{}
		}
	}
	if len(a.samples) == cap(a.samples) {
		copy(a.samples, a.samples[1:])
		a.samples = a.samples[:len(a.samples)-1]
	}
	a.samples = append(a.samples, Sample{clean(rx), clean(tx), now})
	cutoff := now.Add(-a.history - time.Second)
	first := 0
	for first+1 < len(a.samples) && a.samples[first+1].Timestamp.Before(cutoff) {
		first++
	}
	if first > 0 {
		copy(a.samples, a.samples[first:])
		a.samples = a.samples[:len(a.samples)-first]
	}
}

func clean(v float64) float64 {
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func (a *Animator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.samples = a.samples[:0]
	a.smoothedMax = 0
	a.lastScale = time.Time{}
}

func (a *Animator) HasData() bool { a.mu.Lock(); defer a.mu.Unlock(); return len(a.samples) >= 2 }

// HasMotion allows a completely flat, settled wave to sleep until data changes.
func (a *Animator) HasMotion(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.samples) < 2 {
		return false
	}
	target := 1000.0
	for _, sample := range a.samples {
		if now.Sub(sample.Timestamp) <= a.history {
			target = math.Max(target, math.Max(sample.Rx, sample.Tx))
		}
	}
	if !a.fixedScale && a.smoothedMax > target*1.15*1.01 {
		return true
	}
	first := a.samples[0]
	for _, s := range a.samples {
		if s.Rx != first.Rx || s.Tx != first.Tx {
			return true
		}
	}
	return (first.Rx > 0 || first.Tx > 0) && now.Sub(first.Timestamp) < a.history
}

func (a *Animator) GetWavePoints(_ int) ([]display.WavePoint, []display.WavePoint) {
	return a.PointsAt(time.Now(), 0)
}

// PointsAt returns borrowed buffers, valid until the next call. A positive
// ceiling selects link-speed scaling; zero selects a slowly decaying scale.
// Scaling uses elapsed time, so changing frame rate does not change its behavior.
func (a *Animator) PointsAt(now time.Time, ceiling float64) ([]display.WavePoint, []display.WavePoint) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rx = a.rx[:0]
	a.tx = a.tx[:0]
	if len(a.samples) < 2 {
		return a.rx, a.tx
	}
	target := 1000.0
	for _, s := range a.samples {
		if now.Sub(s.Timestamp) <= a.history {
			target = math.Max(target, math.Max(s.Rx, s.Tx))
		}
	}
	target *= 1.15
	a.fixedScale = ceiling > 0
	if ceiling > 0 {
		a.smoothedMax = ceiling
	} else if a.smoothedMax == 0 || target > a.smoothedMax {
		a.smoothedMax = target
	} else if !a.lastScale.IsZero() {
		dt := math.Max(0, now.Sub(a.lastScale).Seconds())
		a.smoothedMax = target + (a.smoothedMax-target)*math.Exp(-dt/4)
	}
	a.lastScale = now
	for _, s := range a.samples {
		x := float64(display.Width) * (1 - now.Sub(s.Timestamp).Seconds()/a.history.Seconds())
		a.rx = append(a.rx, display.WavePoint{X: x, Y: math.Min(1, s.Rx/a.smoothedMax)})
		a.tx = append(a.tx, display.WavePoint{X: x, Y: math.Min(1, s.Tx/a.smoothedMax)})
	}
	// Hold the most recent sample at the live edge until the next 1 Hz sample.
	n := len(a.rx) - 1
	if a.rx[n].X < float64(display.Width) {
		a.rx = append(a.rx, display.WavePoint{X: display.Width, Y: a.rx[n].Y})
		a.tx = append(a.tx, display.WavePoint{X: display.Width, Y: a.tx[n].Y})
	}
	return a.rx, a.tx
}

func (a *Animator) Scale() float64 { a.mu.Lock(); defer a.mu.Unlock(); return a.smoothedMax }
func (a *Animator) GetConfig() (int, float64) {
	return display.Width, float64(display.Width) / a.history.Seconds()
}
