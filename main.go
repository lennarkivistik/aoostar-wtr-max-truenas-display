package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-wtr-display/dashboard"
	"go-wtr-display/display"
	"go-wtr-display/network"
	"go-wtr-display/wave"
)

// version is set via ldflags at build time
var version = "dev"

func main() {
	// CLI flags
	device := flag.String("device", "", "Serial device path (e.g., /dev/ttyACM0)")
	on := flag.Bool("on", false, "Turn display on")
	off := flag.Bool("off", false, "Turn display off")
	iface := flag.String("iface", "", "Network interface to monitor (e.g., eth0)")
	listIfaces := flag.Bool("list-ifaces", false, "List available network interfaces")
	showVersion := flag.Bool("version", false, "Show version and exit")
	samples := flag.Int("samples", 20, "Legacy alias for wave-history in seconds (5-300)")
	waveFps := flag.Int("wave-fps", 4, "Sparkline animation FPS (default: 4)")
	frameInterval := flag.String("frame-interval", "", "Override update interval (e.g. 5s, 500ms). Takes precedence over wave-fps.")
	autoMode := flag.Bool("auto", false, "Legacy automatic night schedule (23:00-10:00); profiles always manage rendering cost")
	noCache := flag.Bool("no-cache", false, "Disable frame caching (sends full frames)")
	offTime := flag.String("off-time", "", "Time to turn display off (HH:MM, e.g., 22:00)")
	onTime := flag.String("on-time", "", "Time to turn display on (HH:MM, e.g., 08:00)")

	// Notification flags (sends to running instance)
	notify := flag.String("notify", "", "Send notification to running display")
	notifyType := flag.String("notify-type", "info", "Notification type: info, warning, danger")
	notifyDuration := flag.Int("notify-duration", 10, "Notification duration in seconds (0 = persistent)")

	// Screenshot flag (requests from running instance)
	screenshot := flag.String("screenshot", "", "Save screenshot from running display to file (PNG)")
	benchDisplay := flag.String("bench-display", "", "Benchmark display updates: full, dirty, noop")
	benchFrames := flag.Int("bench-frames", 20, "Number of frames for --bench-display")
	benchJSON := flag.Bool("bench-json", false, "Output --bench-display results as JSON")

	fixedView := flag.String("view", "", "Pin one live view: network, storage, system, drives")
	views := flag.String("views", "network,storage,system,drives", "Ordered rotating views (comma-separated)")
	dwell := flag.Duration("view-duration", 10*time.Second, "Time per rotating page (3s-1h)")
	pool := flag.String("pool", "", "Storage pool to display (default: all in rotation, first when fixed)")
	profile := flag.String("profile", "balanced", "Resource profile: eco, balanced, smooth")
	waveStyle := flag.String("wave-style", "filled", "Wave appearance: filled, line, off")
	waveHistory := flag.Duration("wave-history", 20*time.Second, "Visible network history (5s-5m)")
	waveScale := flag.String("wave-scale", "adaptive", "Wave scale: adaptive or link")
	nightMode := flag.String("night-mode", "auto", "Night schedule: auto (legacy --auto or explicit times), on, off")
	preview := flag.String("preview", "", "Write demo PNGs for every view to this directory; no device required")

	flag.Parse()
	providedFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		providedFlags[f.Name] = true
	})

	options := dashboard.DefaultOptions()
	if providedFlags["view"] && providedFlags["views"] {
		fatal(fmt.Errorf("use --view or --views, not both"))
	}
	if err := options.SetViews(*fixedView, *views); err != nil {
		fatal(err)
	}
	options.Dwell, options.Pool, options.Profile = *dwell, *pool, *profile
	options.WaveStyle, options.WaveHistory, options.WaveScale = *waveStyle, *waveHistory, *waveScale
	if providedFlags["samples"] && !providedFlags["wave-history"] {
		if *samples < 5 || *samples > 300 {
			fatal(fmt.Errorf("samples must be between 5 and 300"))
		}
		options.WaveHistory = time.Duration(*samples) * time.Second
	}
	if *frameInterval != "" {
		interval, err := time.ParseDuration(*frameInterval)
		if err != nil || interval <= 0 {
			fatal(fmt.Errorf("frame-interval must be a positive duration"))
		}
		options.FrameInterval = interval
	} else if providedFlags["wave-fps"] {
		if *waveFps < 1 || *waveFps > 10 {
			fatal(fmt.Errorf("wave-fps must be between 1 and 10"))
		}
		options.FrameInterval = time.Second / time.Duration(*waveFps)
	}
	if err := options.Validate(); err != nil {
		fatal(err)
	}
	if *nightMode != "auto" && *nightMode != "on" && *nightMode != "off" {
		fatal(fmt.Errorf("night-mode must be auto, on, or off"))
	}
	var schedule *dashboard.Schedule
	if *nightMode != "off" {
		if *autoMode || *nightMode == "on" {
			if *offTime == "" {
				*offTime = "23:00"
			}
			if *onTime == "" {
				*onTime = "10:00"
			}
		}
		if *offTime != "" || *onTime != "" {
			schedule = dashboard.ParseSchedule(*offTime, *onTime)
			if schedule == nil {
				fatal(fmt.Errorf("supply valid, different off-time and on-time in HH:MM format"))
			}
		}
	}
	if *preview != "" {
		if err := dashboard.WritePreviews(*preview, options); err != nil {
			fatal(err)
		}
		fmt.Printf("Demo previews saved to %s\n", *preview)
		return
	}

	// Handle notification sending (client mode)
	if *notify != "" {
		if err := dashboard.SendNotification(*notify, *notifyType, *notifyDuration); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Notification sent")
		return
	}

	// Handle screenshot request (client mode)
	if *screenshot != "" {
		if err := dashboard.RequestScreenshot(*screenshot); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Screenshot saved to %s\n", *screenshot)
		return
	}

	if *showVersion {
		fmt.Printf("wtr-display %s\n", version)
		return
	}

	if *listIfaces {
		interfaces, err := network.ListInterfaces()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Available network interfaces:")
		for _, iface := range interfaces {
			fmt.Printf("  - %s\n", iface)
		}
		return
	}

	hasAction := *on || *off || *iface != "" || *benchDisplay != ""
	if !hasAction {
		fmt.Println("WTR Display - rotating and fixed TrueNAS views")
		fmt.Println("Rotate: wtr-display --iface eth0")
		fmt.Println("Fixed:  wtr-display --iface eth0 --view network")
		fmt.Println("Pool:   wtr-display --iface eth0 --view storage --pool tank")
		fmt.Println("Demo:   wtr-display --preview ./previews")
		flag.PrintDefaults()
		return
	}

	// Open display
	var d *display.Display
	var err error

	if *device != "" {
		fmt.Fprintf(os.Stderr, "Opening device: %s\n", *device)
		d, err = display.Open(*device)
	} else {
		fmt.Fprintln(os.Stderr, "Auto-detecting display...")
		d, err = display.FindAndOpen()
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()

	if *on {
		fmt.Println("Turning display on...")
		if err := d.On(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Display is now on")
		return
	}

	if *off {
		fmt.Println("Turning display off...")
		if err := d.Off(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Display is now off")
		return
	}

	if *benchDisplay != "" {
		if *benchFrames <= 0 {
			fmt.Println("Error: bench-frames must be > 0")
			os.Exit(1)
		}
		if err := runDisplayBenchmark(d, *benchDisplay, *benchFrames, *benchJSON); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *iface != "" {
		if *noCache {
			d.SetCacheEnabled(false)
		}
		if err := runDashboard(d, *iface, options, schedule); err != nil {
			fatal(err)
		}
	}
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "Error:", err); os.Exit(1) }

type storageSnapshot struct {
	pools []network.ZFSPool
	disks []network.DiskSlot
	zfs   *network.ZFSStatus
}

func runDashboard(d *display.Display, iface string, options dashboard.Options, schedule *dashboard.Schedule) error {
	animator := wave.NewAnimatorWithHistory(options.WaveHistory)
	state := dashboard.NewState(iface, animator)
	notifyServer, err := dashboard.NewNotifyServer(state)
	if err != nil {
		return err
	}
	notifyServer.Start()
	defer notifyServer.Stop()
	fmt.Printf("Views: %v | fixed: %t | profile: %s | socket: %s\n", options.Views, options.Fixed, options.Profile, dashboard.DefaultSocketPath)

	netMonitor := network.NewMonitor(iface)
	cpuMonitor := network.NewCPUMonitor()
	refreshIdentity := func() {
		state.IPAddress, _ = network.GetInterfaceIP(iface)
		state.LinkSpeed = network.GetLinkSpeed(iface)
		state.LinkState = network.GetLinkState(iface)
		state.Uptime = network.GetUptime()
	}
	refreshSystem := func() {
		state.Memory = network.GetMemoryInfo()
		state.ARC = network.GetARCInfo()
		state.Temperatures = network.GetTemperatures(state.Disks)
	}
	refreshIdentity()
	refreshSystem()
	netMonitor.GetThroughput()
	cpuMonitor.GetUsage()
	state.LoggedInUsers = network.GetLoggedInUsers()

	// Slow commands run sequentially in one worker, outside the render loop.
	// A one-element channel coalesces results rather than queuing stale snapshots.
	updates := make(chan storageSnapshot, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			update := storageSnapshot{network.GetZFSPools(), network.GetDiskSlots(), network.GetZFSStatus()}
			select {
			case updates <- update:
			case <-done:
				return
			}
			select {
			case <-ticker.C:
			case <-done:
				return
			}
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	sampleTicker := time.NewTicker(time.Second)
	defer sampleTicker.Stop()
	slowTicker := time.NewTicker(5 * time.Second)
	defer slowTicker.Stop()
	identityTicker := time.NewTicker(30 * time.Second)
	defer identityTicker.Stop()
	scheduleTicker := time.NewTicker(time.Minute)
	defer scheduleTicker.Stop()
	loginTicker := time.NewTicker(10 * time.Second)
	defer loginTicker.Stop()
	started := time.Now()
	page := options.PageAt(started, started, options.Pages(dashboard.PoolNames(state)))
	interval := dashboard.RefreshInterval(state, page, options, 0)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	renderer := dashboard.NewFrameRenderer()
	displayOff := schedule != nil && schedule.IsNightMode()
	force := true
	if displayOff {
		if err := d.Off(); err != nil {
			return err
		}
	} else {
		if err := d.On(); err != nil {
			return err
		}
	}
	var lastSend time.Duration
	var lastHealth dashboard.Health
	render := func(now time.Time) {
		if displayOff {
			return
		}
		state.ClearExpiredNotification()
		lastHealth = dashboard.CurrentHealth(state)
		page = options.PageAt(started, now, options.Pages(dashboard.PoolNames(state)))
		if img, changed := renderer.Frame(state, page, options, now, force); changed {
			stats, err := d.SendImage(img)
			if err != nil {
				// The device may have accepted only part of a failed frame.
				d.ClearCache()
				force = true
				fmt.Fprintf(os.Stderr, "Display update failed: %v\n", err)
				lastSend = time.Second
			} else {
				force = false
				state.SetLastFrame(img)
				// Full-page transitions are infrequent and should not permanently
				// throttle ordinary wave updates. Partial transfers set the budget.
				if stats.ChangedChunks > 0 && stats.ChangedChunks < stats.TotalChunks/2 {
					lastSend = (3*lastSend + stats.SendTime) / 4
				}
			}
		}
		next := dashboard.RefreshInterval(state, page, options, lastSend)
		if next != interval {
			ticker.Reset(next)
			interval = next
		}
	}
	render(started)
	started = time.Now() // Give the first visible page its full dwell after transfer.
	// Alarms and page changes must not wait behind a long manual frame interval.
	renderImportant := func() {
		now := time.Now()
		nextPage := options.PageAt(started, now, options.Pages(dashboard.PoolNames(state)))
		if nextPage != page || dashboard.CurrentHealth(state) != lastHealth {
			render(now)
		}
	}
	for {
		select {
		case <-signals:
			return nil
		case <-ticker.C:
			render(time.Now())
		case <-state.NotificationsChanged:
			render(time.Now())
		case update := <-updates:
			state.PoolReadFailed = update.pools == nil
			if update.pools != nil {
				state.Pools = update.pools
			}
			state.Disks = update.disks
			state.ZFSReadFailed = update.zfs == nil
			if update.zfs != nil {
				state.ZFSStatus = update.zfs
			}
			if !displayOff {
				state.Temperatures = network.GetTemperatures(state.Disks)
			}
			renderImportant()
		case <-sampleTicker.C:
			if displayOff {
				continue
			}
			throughput, err := netMonitor.GetThroughput()
			state.NetworkAvailable = err == nil
			if err == nil {
				animator.AddSample(throughput.RxBytesPerSec, throughput.TxBytesPerSec)
				// A short exponential average stabilizes digits without ten seconds of lag.
				state.DisplayRxSpeed += 0.5 * (throughput.RxBytesPerSec - state.DisplayRxSpeed)
				state.DisplayTxSpeed += 0.5 * (throughput.TxBytesPerSec - state.DisplayTxSpeed)
			} else {
				state.DisplayRxSpeed = 0
				state.DisplayTxSpeed = 0
				animator.Reset()
			}
			state.DisplayCPU = cpuMonitor.GetUsage()
			state.CPUAvailable = !math.IsNaN(state.DisplayCPU)
			// Link state is cheap to read and must not wait for the metadata poll.
			state.LinkState = network.GetLinkState(iface)
			state.ClearExpiredNotification()
			renderImportant()
		case <-slowTicker.C:
			if !displayOff {
				refreshSystem()
			}
		case <-identityTicker.C:
			if !displayOff {
				refreshIdentity()
			}
		case <-loginTicker.C:
			state.LoggedInUsers = network.GetLoggedInUsers()
		case <-scheduleTicker.C:
			shouldBeOff := schedule != nil && schedule.IsNightMode()
			if shouldBeOff == displayOff {
				continue
			}
			if shouldBeOff {
				if err := d.Off(); err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				displayOff = true
				ticker.Reset(time.Minute)
			} else {
				if err := d.On(); err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				displayOff = false
				force = true
				d.ClearCache()
				animator.Reset()
				netMonitor = network.NewMonitor(iface)
				cpuMonitor = network.NewCPUMonitor()
				netMonitor.GetThroughput()
				cpuMonitor.GetUsage()
				state.NetworkAvailable = false
				state.DisplayRxSpeed = 0
				state.DisplayTxSpeed = 0
				state.CPUAvailable = false
				refreshIdentity()
				refreshSystem()
				started = time.Now()
				interval = dashboard.RefreshInterval(state, page, options, 0)
				ticker.Reset(interval)
				render(time.Now())
			}
		}
	}
}

type displayBenchResult struct {
	Mode            string  `json:"mode"`
	Frames          int     `json:"frames"`
	WallMS          float64 `json:"wall_ms"`
	AvgFrameMS      float64 `json:"avg_frame_ms"`
	AvgConvertMS    float64 `json:"avg_convert_ms"`
	AvgDiffMS       float64 `json:"avg_diff_ms"`
	AvgSendMS       float64 `json:"avg_send_ms"`
	ChangedChunks   int     `json:"changed_chunks"`
	BytesWritten    int     `json:"bytes_written"`
	SerialWrites    int     `json:"serial_writes"`
	AvgChunksPerRun float64 `json:"avg_chunks_per_frame"`
	AvgBytesPerRun  float64 `json:"avg_bytes_per_frame"`
	AvgWritesPerRun float64 `json:"avg_writes_per_frame"`
}

func runDisplayBenchmark(d *display.Display, mode string, frames int, jsonOut bool) error {
	img := display.NewDisplayImage()
	base := display.NewDisplayImage()
	display.FillImage(base, color.RGBA{15, 23, 42, 255})
	display.CopyImage(img, base)

	switch mode {
	case "full":
		d.SetCacheEnabled(false)
	case "dirty", "noop":
		d.SetCacheEnabled(true)
		d.ClearCache()
		if _, err := d.SendImage(base); err != nil {
			return fmt.Errorf("failed to seed benchmark frame: %w", err)
		}
	default:
		return fmt.Errorf("unknown bench-display mode %q (use full, dirty, noop)", mode)
	}

	var result displayBenchResult
	result.Mode = mode
	result.Frames = frames

	start := time.Now()
	for i := 0; i < frames; i++ {
		switch mode {
		case "full":
			if i%2 == 0 {
				display.FillImage(img, color.RGBA{15, 23, 42, 255})
			} else {
				display.FillImage(img, color.RGBA{16, 24, 43, 255})
			}
		case "dirty":
			display.CopyImage(img, base)
			c := color.RGBA{34, 211, 238, 255}
			if i%2 == 1 {
				c = color.RGBA{96, 165, 250, 255}
			}
			display.DrawFilledRect(img, 430, 168, 100, 40, c)
		case "noop":
			display.CopyImage(img, base)
		}

		stats, err := d.SendImage(img)
		if err != nil {
			return fmt.Errorf("frame %d failed: %w", i+1, err)
		}
		result.ChangedChunks += stats.ChangedChunks
		result.BytesWritten += stats.BytesWritten
		result.SerialWrites += stats.Writes
		result.AvgConvertMS += durationMS(stats.ConvertTime)
		result.AvgDiffMS += durationMS(stats.DiffTime)
		result.AvgSendMS += durationMS(stats.SendTime)
	}
	result.WallMS = durationMS(time.Since(start))
	result.AvgFrameMS = result.WallMS / float64(frames)
	result.AvgConvertMS /= float64(frames)
	result.AvgDiffMS /= float64(frames)
	result.AvgSendMS /= float64(frames)
	result.AvgChunksPerRun = float64(result.ChangedChunks) / float64(frames)
	result.AvgBytesPerRun = float64(result.BytesWritten) / float64(frames)
	result.AvgWritesPerRun = float64(result.SerialWrites) / float64(frames)

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Benchmark mode=%s frames=%d\n", result.Mode, result.Frames)
	fmt.Printf("Wall: %.1f ms total, %.2f ms/frame\n", result.WallMS, result.AvgFrameMS)
	fmt.Printf("CPU: convert %.2f ms/frame, diff %.2f ms/frame, send wait %.2f ms/frame\n",
		result.AvgConvertMS, result.AvgDiffMS, result.AvgSendMS)
	fmt.Printf("Display I/O: %.1f chunks/frame, %.0f bytes/frame, %.1f writes/frame\n",
		result.AvgChunksPerRun, result.AvgBytesPerRun, result.AvgWritesPerRun)
	return nil
}

func durationMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
