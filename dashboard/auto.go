package dashboard

import "time"

// RefreshInterval limits rendering by visibility, CPU load and observed I/O.
// Manual FPS/interval options set a requested rate, never bypass the load guard.
func RefreshInterval(s *State, p Page, o Options, lastSend time.Duration) time.Duration {
	interval := time.Second
	if p.View == ViewNetwork && o.WaveStyle != "off" && !CurrentHealth(s).Critical && s.GetActiveNotification() == nil {
		switch o.Profile {
		case "balanced":
			interval = 250 * time.Millisecond
		case "smooth":
			interval = 125 * time.Millisecond
		}
		if o.FrameInterval > 0 {
			interval = o.FrameInterval
		}
		if !s.NetworkAvailable || s.DisplayRxSpeed+s.DisplayTxSpeed < 1024 {
			if interval < 500*time.Millisecond {
				interval = 500 * time.Millisecond
			}
		}
	} else if o.FrameInterval > interval {
		interval = o.FrameInterval
	}
	if !s.CPUAvailable || s.DisplayCPU > 20 {
		if interval < time.Second {
			interval = time.Second
		}
	}
	if s.DisplayCPU > 75 && interval < 2*time.Second {
		interval = 2 * time.Second
	}
	// Leave at least as much idle time as time spent transferring a frame.
	if floor := 2 * lastSend; floor > interval {
		interval = floor
	}
	return interval
}
