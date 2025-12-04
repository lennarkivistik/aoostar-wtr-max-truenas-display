package main

import (
	"flag"
	"fmt"
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
	samples := flag.Int("samples", 5, "Number of samples for sparkline history (default: 5)")
	waveFps := flag.Int("wave-fps", 4, "Sparkline animation FPS (default: 4)")
	frameInterval := flag.String("frame-interval", "", "Override update interval (e.g. 5s, 500ms). Takes precedence over wave-fps.")
	autoMode := flag.Bool("auto", false, "Enable smart auto-scaling FPS (based on load/traffic) and default night mode (23:00-10:00)")
	noCache := flag.Bool("no-cache", false, "Disable frame caching (sends full frames)")
	offTime := flag.String("off-time", "", "Time to turn display off (HH:MM, e.g., 22:00)")
	onTime := flag.String("on-time", "", "Time to turn display on (HH:MM, e.g., 08:00)")

	// Notification flags (sends to running instance)
	notify := flag.String("notify", "", "Send notification to running display")
	notifyType := flag.String("notify-type", "info", "Notification type: info, warning, danger")
	notifyDuration := flag.Int("notify-duration", 10, "Notification duration in seconds (0 = persistent)")

	// Screenshot flag (requests from running instance)
	screenshot := flag.String("screenshot", "", "Save screenshot from running display to file (PNG)")

	flag.Parse()

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

	hasAction := *on || *off || *iface != ""
	if !hasAction {
		fmt.Println("WTR Display - TrueNAS Dashboard")
		fmt.Println("\nUsage: wtr-display [options]")
		fmt.Println("\nCommands:")
		fmt.Println("  --on                Turn display on")
		fmt.Println("  --off               Turn display off")
		fmt.Println("  --iface eth0        Monitor network interface")
		fmt.Println("  --list-ifaces       List available interfaces")
		fmt.Println("\nNotifications (send to running instance):")
		fmt.Println("  --notify \"message\"  Send notification to display")
		fmt.Println("  --notify-type TYPE  Type: info, warning, danger (default: info)")
		fmt.Println("  --notify-duration N Duration in seconds (default: 10, 0=persistent)")
		fmt.Println("\nScreenshot:")
		fmt.Println("  --screenshot FILE   Save screenshot from running display (PNG)")
		fmt.Println("\nOptions:")
		fmt.Println("  --wave-fps N        Sparkline animation FPS (default: 4)")
		fmt.Println("  --samples N         Sparkline history samples (default: 5)")
		fmt.Println("  --auto              Auto Mode (Smart FPS + Night Mode)")
		fmt.Println("  --off-time HH:MM    Night mode start (e.g., 22:00)")
		fmt.Println("  --on-time HH:MM     Night mode end (e.g., 08:00)")
		fmt.Println("  --no-cache          Disable frame caching (slower)")
		fmt.Println("\nExamples:")
		fmt.Println("  wtr-display --iface eth0")
		fmt.Println("  wtr-display --notify \"Backup complete\" --notify-type info")
		fmt.Println("  wtr-display --notify \"Drive failure!\" --notify-type danger --notify-duration 0")
		fmt.Println("  wtr-display --screenshot /tmp/display.png")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Open display
	var d *display.Display
	var err error

	if *device != "" {
		fmt.Printf("Opening device: %s\n", *device)
		d, err = display.Open(*device)
	} else {
		fmt.Println("Auto-detecting display...")
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

	if *iface != "" {
		fmt.Printf("Starting dashboard for interface: %s\n", *iface)
		if *noCache {
			d.SetCacheEnabled(false)
			fmt.Println("Frame caching disabled (full frame updates)")
		}

		// Parse schedule times
		if *autoMode {
			if *offTime == "" {
				*offTime = "23:00"
			}
			if *onTime == "" {
				*onTime = "10:00"
			}
			fmt.Printf("Auto Mode: Default schedule enabled (Off %s, On %s)\n", *offTime, *onTime)
		}

		schedule := dashboard.ParseSchedule(*offTime, *onTime)
		if schedule != nil {
			fmt.Printf("Night mode: display off %s, on %s\n", *offTime, *onTime)
			now := time.Now()
			fmt.Printf("Current system time: %s\n", now.Format("15:04:05"))
			if schedule.IsNightMode() {
				fmt.Println("Status: Inside night mode window (should be OFF)")
			} else {
				fmt.Println("Status: Outside night mode window (should be ON)")
			}
		}

		// Calculate initial animation interval
		var animDuration time.Duration
		if *frameInterval != "" {
			var err error
			animDuration, err = time.ParseDuration(*frameInterval)
			if err != nil {
				fmt.Printf("Error parsing frame-interval: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Update interval set to %v\n", animDuration)
		} else {
			if *waveFps <= 0 {
				fmt.Println("Error: wave-fps must be > 0")
				os.Exit(1)
			}
			animDuration = time.Second / time.Duration(*waveFps)
		}

		runDashboard(d, *iface, *samples, animDuration, schedule, *autoMode)
	}
}

func runDashboard(d *display.Display, iface string, samples int, animDuration time.Duration, schedule *dashboard.Schedule, autoMode bool) {
	// Initialize wave animator
	animator := wave.NewAnimator(samples)

	// Initialize state
	state := dashboard.NewState(iface, animator)

	// Start notification server (for external --notify commands)
	notifyServer, err := dashboard.NewNotifyServer(state)
	if err != nil {
		fmt.Printf("Warning: notification server failed to start: %v\n", err)
	} else {
		notifyServer.Start()
		defer notifyServer.Stop()
		fmt.Printf("Notification socket: %s\n", dashboard.DefaultSocketPath)
	}

	// Reusable image buffers
	bgImg := display.NewDisplayImage()
	img := display.NewDisplayImage()

	// Renderer
	renderer := display.NewTextRenderer()

	// Monitors
	netMonitor := network.NewMonitor(iface)
	cpuMonitor := network.NewCPUMonitor()

	// Initial fetch
	state.IPAddress, _ = network.GetInterfaceIP(iface)
	state.LinkSpeed = network.GetLinkSpeed(iface)
	state.Uptime = network.GetUptime()
	state.Memory = network.GetMemoryInfo()
	state.Pools = network.GetZFSPools()

	// Render static layer
	dashboard.RenderStaticLayer(bgImg, renderer, state)

	fmt.Printf("TrueNAS: %s | IP: %s | Link: %s\n",
		state.TrueNASVersion, state.IPAddress, network.FormatLinkSpeed(state.LinkSpeed))

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Tickers
	sampleTicker := time.NewTicker(1 * time.Second)
	slowTicker := time.NewTicker(5 * time.Second)
	staticTicker := time.NewTicker(30 * time.Second)
	scheduleTicker := time.NewTicker(1 * time.Minute)
	loginTicker := time.NewTicker(10 * time.Second) // Check logged-in users
	zfsTicker := time.NewTicker(60 * time.Second)   // Check ZFS status
	defer sampleTicker.Stop()
	defer slowTicker.Stop()
	defer staticTicker.Stop()
	defer scheduleTicker.Stop()
	defer loginTicker.Stop()
	defer zfsTicker.Stop()

	// Animation ticker
	animTicker := time.NewTicker(animDuration)
	defer animTicker.Stop()

	// Initial baseline
	netMonitor.GetThroughput()
	cpuMonitor.GetUsage()

	// Initial notification state
	state.LoggedInUsers = network.GetLoggedInUsers()
	state.ZFSStatus = network.GetZFSStatus()
	state.UpdateNotificationFromZFS()

	displayOff := false

	// Initial schedule check
	if schedule != nil && schedule.IsNightMode() {
		displayOff = true
		d.Off()
		fmt.Println("\nNight mode active - display off")
	} else {
		if err := d.On(); err != nil {
			fmt.Printf("Warning: failed to turn on display: %v\n", err)
		}
		dashboard.RenderAndSend(d, img, bgImg, renderer, state)
	}

	currentAnimDuration := animDuration

	for {
		select {
		case <-sigChan:
			fmt.Println("\nShutting down...")
			return

		case <-scheduleTicker.C:
			if schedule != nil {
				shouldBeOff := schedule.IsNightMode()
				if shouldBeOff && !displayOff {
					displayOff = true
					d.Off()
					fmt.Printf("\n[%s] Night mode - display off\n", time.Now().Format("15:04"))
				} else if !shouldBeOff && displayOff {
					displayOff = false
					d.On()
					d.ClearCache()
					fmt.Printf("\n[%s] Night mode ended - display on\n", time.Now().Format("15:04"))
				}
			}

		case <-sampleTicker.C:
			if displayOff {
				continue
			}

			if throughput, err := netMonitor.GetThroughput(); err == nil {
				animator.AddSample(throughput.RxBytesPerSec, throughput.TxBytesPerSec)

				state.RecentRx = append(state.RecentRx, throughput.RxBytesPerSec)
				state.RecentTx = append(state.RecentTx, throughput.TxBytesPerSec)
				if len(state.RecentRx) > 10 {
					state.RecentRx = state.RecentRx[1:]
					state.RecentTx = state.RecentTx[1:]
				}

				state.DisplayRxSpeed = dashboard.Average(state.RecentRx)
				state.DisplayTxSpeed = dashboard.Average(state.RecentTx)
			}

			state.DisplayCPU = cpuMonitor.GetUsage()
			state.Alerts = state.CheckAlerts()

			fmt.Printf("\r↓ %-12s ↑ %-12s CPU: %5.1f%%  ",
				network.FormatBits(state.DisplayRxSpeed),
				network.FormatBits(state.DisplayTxSpeed),
				state.DisplayCPU)

		case <-animTicker.C:
			if displayOff {
				continue
			}
			// Clear expired notifications
			state.ClearExpiredNotification()
			dashboard.RenderAndSend(d, img, bgImg, renderer, state)

		case <-slowTicker.C:
			if displayOff {
				continue
			}
			state.Memory = network.GetMemoryInfo()
			state.Pools = network.GetZFSPools()

			dashboard.RenderStaticLayer(bgImg, renderer, state)

			if autoMode {
				targetDuration := dashboard.CalculateAutoFPS(state)
				if targetDuration != currentAnimDuration {
					animTicker.Stop()
					animTicker = time.NewTicker(targetDuration)
					currentAnimDuration = targetDuration
				}
			}

		case <-staticTicker.C:
			if displayOff {
				continue
			}
			state.IPAddress, _ = network.GetInterfaceIP(iface)
			state.LinkSpeed = network.GetLinkSpeed(iface)
			state.Uptime = network.GetUptime()

			dashboard.RenderStaticLayer(bgImg, renderer, state)

		case <-loginTicker.C:
			// Update logged-in users (lightweight 'who' command)
			state.LoggedInUsers = network.GetLoggedInUsers()

		case <-zfsTicker.C:
			// Update ZFS status and notifications
			state.ZFSStatus = network.GetZFSStatus()
			state.UpdateNotificationFromZFS()
			state.ClearExpiredNotification()
		}
	}
}
