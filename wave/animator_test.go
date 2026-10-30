package wave

import (
	"math"
	"testing"
	"time"
)

func TestScaleIndependentOfFrameRate(t *testing.T) {
	start := time.Unix(0, 0)
	makeAnimator := func() *Animator {
		a := NewAnimator(5)
		a.AddSampleAt(100000, 0, start)
		a.AddSampleAt(1000, 0, start.Add(time.Second))
		a.PointsAt(start.Add(time.Second), 0)
		for i := 2; i <= 6; i++ {
			a.AddSampleAt(1000, 0, start.Add(time.Duration(i)*time.Second))
		}
		return a
	}
	slow, fast := makeAnimator(), makeAnimator()
	// The old high sample has left the visible window. Integrate the same second.
	slow.PointsAt(start.Add(7*time.Second), 0)
	for i := 1; i <= 60; i++ {
		fast.PointsAt(start.Add(time.Second+time.Duration(i)*100*time.Millisecond), 0)
	}
	// Start both from an identical settled target, then compare another second.
	slow.smoothedMax = 10000
	fast.smoothedMax = 10000
	slow.lastScale = start.Add(7 * time.Second)
	fast.lastScale = slow.lastScale
	slow.PointsAt(start.Add(8*time.Second), 0)
	for i := 1; i <= 10; i++ {
		fast.PointsAt(start.Add(7*time.Second+time.Duration(i)*100*time.Millisecond), 0)
	}
	if math.Abs(slow.Scale()-fast.Scale()) > 0.00001 {
		t.Fatalf("scales differ: %v / %v", slow.Scale(), fast.Scale())
	}
}

func TestHistoryBoundedAndGapsReset(t *testing.T) {
	a := NewAnimator(20)
	start := time.Unix(0, 0)
	for i := 0; i < 1000; i++ {
		a.AddSampleAt(float64(i), 0, start.Add(time.Duration(i)*time.Second))
	}
	if len(a.samples) > 24 {
		t.Fatalf("unbounded samples: %d", len(a.samples))
	}
	a.AddSampleAt(10, 20, start.Add(2000*time.Second))
	if a.HasData() {
		t.Fatal("gap retained stale history")
	}
}

func TestFlatWaveSleepsAndLinkScaleIsFixed(t *testing.T) {
	a := NewAnimator(5)
	start := time.Unix(0, 0)
	for i := 0; i <= 6; i++ {
		a.AddSampleAt(100, 50, start.Add(time.Duration(i)*time.Second))
	}
	if a.HasMotion(start.Add(6 * time.Second)) {
		t.Fatal("settled flat wave still animates")
	}
	rx, tx := a.PointsAt(start.Add(6*time.Second), 1000)
	if rx[len(rx)-1].Y != 0.1 || tx[len(tx)-1].Y != 0.05 {
		t.Fatal("incorrect link scaling")
	}
}

func TestFixedLinkScaleCanSleep(t *testing.T) {
	a := NewAnimator(5)
	now := time.Unix(1000, 0)
	for i := 0; i < 8; i++ {
		a.AddSampleAt(1000, 500, now.Add(time.Duration(i-7)*time.Second))
	}
	a.PointsAt(now, 312500000)
	if a.HasMotion(now) {
		t.Fatal("fixed link scale kept a flat wave animating")
	}
}
