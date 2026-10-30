// Command showcase renders a deterministic PNG sequence for the README GIF.
// It uses only synthetic data; it never opens the display or samples the host.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"go-wtr-display/dashboard"
)

func main() {
	out := flag.String("out", "build/showcase", "output frame directory")
	flag.Parse()
	if err := render(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func render(out string) error {
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	options := dashboard.DefaultOptions()
	start := time.Unix(1000, 0)
	state := dashboard.DemoState(start, options.WaveHistory)
	renderer := dashboard.NewFrameRenderer()
	type scene struct {
		view          dashboard.View
		pool, seconds int
		fixed, fault  bool
		style         string
	}
	scenes := []scene{
		{dashboard.ViewNetwork, 0, 6, false, false, "filled"},
		{dashboard.ViewStorage, 0, 4, false, false, "filled"},
		{dashboard.ViewStorage, 1, 4, false, false, "filled"},
		{dashboard.ViewSystem, 0, 4, false, false, "filled"},
		{dashboard.ViewDrives, 0, 4, false, false, "filled"},
		{dashboard.ViewNetwork, 0, 4, true, false, "filled"},
		{dashboard.ViewNetwork, 0, 4, true, false, "line"},
		{dashboard.ViewNetwork, 0, 3, true, true, "line"},
	}
	frame := 0
	for _, scene := range scenes {
		options.Fixed, options.WaveStyle = scene.fixed, scene.style
		if scene.fixed {
			options.Views = []dashboard.View{scene.view}
		}
		if scene.fault {
			state.ZFSStatus.PoolStates["archive"] = "DEGRADED"
			state.ZFSStatus.DegradedPools = []string{"archive"}
		}
		for i := 0; i < scene.seconds*4; i++ {
			now := start.Add(time.Duration(frame) * 250 * time.Millisecond)
			if frame%4 == 0 {
				t := float64(frame)/4 + options.WaveHistory.Seconds() + 1
				state.DisplayRxSpeed = 18e6 + 14e6*math.Sin(t*0.54) + 3e6*math.Sin(t*1.8)
				state.DisplayTxSpeed = 6e6 + 5e6*math.Sin(t*0.62+1.5)
				state.WaveAnimator.AddSampleAt(state.DisplayRxSpeed, state.DisplayTxSpeed, now)
			}
			img, _ := renderer.Frame(state, dashboard.Page{View: scene.view, Pool: scene.pool}, options, now, true)
			f, err := os.Create(filepath.Join(out, fmt.Sprintf("%03d.png", frame)))
			if err != nil {
				return err
			}
			err = png.Encode(f, img)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			frame++
		}
	}
	fmt.Printf("Wrote %d synthetic frames to %s\n", frame, out)
	return nil
}
