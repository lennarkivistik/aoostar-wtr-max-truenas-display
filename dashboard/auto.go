package dashboard

import (
	"time"
)

const (
	// Auto mode thresholds
	highLoadCPU      = 20.0
	busyNetworkBytes = 1024 * 1024 // 1 MB/s (8 Mbps)
)

// CalculateAutoFPS determines the optimal update interval based on system load.
func CalculateAutoFPS(state *State) time.Duration {
	totalTraffic := state.DisplayRxSpeed + state.DisplayTxSpeed

	if state.DisplayCPU > highLoadCPU {
		return 1 * time.Second // 1 FPS (High CPU load)
	} else if totalTraffic > busyNetworkBytes {
		return 66 * time.Millisecond // 15 FPS (High network activity)
	} else {
		return 250 * time.Millisecond // 4 FPS (Idle/Normal)
	}
}
