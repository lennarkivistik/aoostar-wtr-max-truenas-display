package dashboard

import (
	"fmt"
	"strings"
	"time"
)

type View string

const (
	ViewNetwork View = "network"
	ViewStorage View = "storage"
	ViewSystem  View = "system"
	ViewDrives  View = "drives"
)

// Options describes presentation independently of data collection and scheduling.
type Options struct {
	Views         []View
	Fixed         bool
	Dwell         time.Duration
	Pool          string
	Profile       string
	WaveStyle     string
	WaveHistory   time.Duration
	WaveScale     string
	FrameInterval time.Duration
}

func DefaultOptions() Options {
	return Options{Views: []View{ViewNetwork, ViewStorage, ViewSystem, ViewDrives},
		Dwell: 10 * time.Second, Profile: "balanced", WaveStyle: "filled",
		WaveHistory: 20 * time.Second, WaveScale: "adaptive"}
}

func (o *Options) SetViews(fixed, playlist string) error {
	o.Fixed = fixed != ""
	if o.Fixed {
		playlist = fixed
	}
	o.Views = nil
	seen := map[View]bool{}
	for _, name := range strings.Split(playlist, ",") {
		view := View(strings.TrimSpace(name))
		switch view {
		case ViewNetwork, ViewStorage, ViewSystem, ViewDrives:
		default:
			return fmt.Errorf("unknown view %q (use network, storage, system, drives)", name)
		}
		if seen[view] {
			return fmt.Errorf("duplicate view %q", view)
		}
		seen[view] = true
		o.Views = append(o.Views, view)
	}
	if o.Fixed && len(o.Views) != 1 {
		return fmt.Errorf("--view accepts one view")
	}
	return nil
}

func (o Options) Validate() error {
	if len(o.Views) == 0 {
		return fmt.Errorf("at least one view is required")
	}
	if o.Dwell < 3*time.Second || o.Dwell > time.Hour {
		return fmt.Errorf("view-duration must be between 3s and 1h")
	}
	if o.WaveHistory < 5*time.Second || o.WaveHistory > 5*time.Minute {
		return fmt.Errorf("wave-history must be between 5s and 5m")
	}
	if o.FrameInterval != 0 && (o.FrameInterval < 100*time.Millisecond || o.FrameInterval > time.Hour) {
		return fmt.Errorf("frame-interval must be between 100ms and 1h")
	}
	switch o.Profile {
	case "eco", "balanced", "smooth":
	default:
		return fmt.Errorf("profile must be eco, balanced, or smooth")
	}
	switch o.WaveStyle {
	case "filled", "line", "off":
	default:
		return fmt.Errorf("wave-style must be filled, line, or off")
	}
	switch o.WaveScale {
	case "adaptive", "link":
	default:
		return fmt.Errorf("wave-scale must be adaptive or link")
	}
	return nil
}

type Page struct {
	View View
	Pool int
}

// Pages keeps extra pools visible in rotation. A fixed view never advances;
// --pool selects a particular pool without relying on discovery order.
func (o Options) Pages(poolNames []string) []Page {
	var pages []Page
	for _, view := range o.Views {
		if view != ViewStorage {
			pages = append(pages, Page{View: view})
			continue
		}
		added := false
		for i, name := range poolNames {
			if o.Pool != "" && name != o.Pool {
				continue
			}
			pages = append(pages, Page{View: view, Pool: i})
			added = true
			if o.Fixed {
				break
			}
		}
		if !added {
			pages = append(pages, Page{View: view, Pool: -1})
		}
	}
	return pages
}

// PageAt uses elapsed monotonic time, so wall-clock changes cannot skip pages.
func (o Options) PageAt(start, now time.Time, pages []Page) Page {
	if len(pages) == 0 {
		return Page{View: ViewNetwork}
	}
	if o.Fixed || now.Before(start) {
		return pages[0]
	}
	return pages[int(now.Sub(start)/o.Dwell)%len(pages)]
}
