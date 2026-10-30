package dashboard

import (
	"bytes"
	"image"
	"testing"
	"time"
)

func TestEveryViewRendersAndStaticFramesSleep(t *testing.T) {
	now := time.Unix(1000, 0)
	o := DefaultOptions()
	for _, view := range o.Views {
		t.Run(string(view), func(t *testing.T) {
			s := DemoState(now, o.WaveHistory)
			r := NewFrameRenderer()
			page := Page{View: view}
			img, changed := r.Frame(s, page, o, now, false)
			if !changed || img == nil || img.Bounds() != image.Rect(0, 0, 960, 376) {
				t.Fatal("missing frame")
			}
			if view != ViewNetwork {
				if _, changed := r.Frame(s, page, o, now.Add(time.Second), false); changed {
					t.Fatal("unchanged static view rendered again")
				}
			}
		})
	}
}

func TestHiddenMetricsDoNotRedrawStorage(t *testing.T) {
	now := time.Unix(1000, 0)
	o := DefaultOptions()
	s := DemoState(now, o.WaveHistory)
	r := NewFrameRenderer()
	p := Page{View: ViewStorage}
	r.Frame(s, p, o, now, false)
	s.DisplayCPU = 18
	s.DisplayRxSpeed = 90_000_000
	if _, changed := r.Frame(s, p, o, now.Add(time.Second), false); changed {
		t.Fatal("hidden values invalidated storage")
	}
	s.Pools[0].Free = "24.2T"
	if _, changed := r.Frame(s, p, o, now.Add(2*time.Second), false); !changed {
		t.Fatal("capacity change was not rendered")
	}
}

func TestScreenshotOwnsCompletedFrame(t *testing.T) {
	s := &State{}
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Pix[0] = 77
	s.SetLastFrame(img)
	img.Pix[0] = 99
	if got := s.GetLastFrame(); got.Pix[0] != 77 {
		t.Fatal("render buffer changed screenshot")
	}
}

func TestCriticalFrameIgnoresInformationalOverlay(t *testing.T) {
	now := time.Unix(1000, 0)
	o := DefaultOptions()
	s := DemoState(now, o.WaveHistory)
	s.ZFSStatus.DegradedPools = []string{"tank"}
	s.ZFSStatus.PoolStates["tank"] = "DEGRADED"
	r := NewFrameRenderer()
	img, _ := r.Frame(s, Page{View: ViewNetwork}, o, now, true)
	before := append([]byte(nil), img.Pix...)
	s.SetActiveNotification(&Notification{Message: "Everything is fine", Theme: ThemeInfo})
	img, _ = r.Frame(s, Page{View: ViewNetwork}, o, now, true)
	if !bytes.Equal(before, img.Pix) {
		t.Fatal("informational notification obscured fault")
	}
}
