// Package network system.go provides system resource monitoring:
// CPU usage, memory usage, and ZFS pool statistics.
package network

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CPUStats holds raw CPU time counters from /proc/stat.
// All values are in "jiffies" (typically 1/100th of a second).
// These are cumulative since boot.
type CPUStats struct {
	User    uint64 // Time spent in user mode
	Nice    uint64 // Time spent in user mode with low priority (nice)
	System  uint64 // Time spent in system mode (kernel)
	Idle    uint64 // Time spent idle
	IOWait  uint64 // Time waiting for I/O to complete
	IRQ     uint64 // Time servicing hardware interrupts
	SoftIRQ uint64 // Time servicing software interrupts
}

// CPUMonitor tracks CPU usage by comparing /proc/stat samples over time.
// Must be called repeatedly to get meaningful usage percentages.
type CPUMonitor struct {
	prevStats *CPUStats // Previous sample for delta calculation
}

// NewCPUMonitor creates a new CPU monitor.
// Call GetUsage() repeatedly at regular intervals for accurate readings.
func NewCPUMonitor() *CPUMonitor {
	return &CPUMonitor{}
}

// GetUsage calculates CPU usage as a percentage (0-100) since the last call.
// Formula: usage = busy_time / total_time * 100
// Where busy = user + nice + system + irq + softirq
// And idle = idle + iowait (iowait is CPU waiting for I/O, not doing work)
// On first call, returns 0 and stores baseline. Subsequent calls return actual usage.
func (c *CPUMonitor) GetUsage() float64 {
	current := c.readCPUStats()
	if current == nil {
		return 0
	}

	if c.prevStats == nil {
		c.prevStats = current
		return 0
	}

	// Calculate deltas for busy time (actual CPU work)
	prevBusy := c.prevStats.User + c.prevStats.Nice + c.prevStats.System + c.prevStats.IRQ + c.prevStats.SoftIRQ
	currBusy := current.User + current.Nice + current.System + current.IRQ + current.SoftIRQ

	// Calculate deltas for idle time (including iowait - CPU waiting for I/O is not "busy")
	prevIdle := c.prevStats.Idle + c.prevStats.IOWait
	currIdle := current.Idle + current.IOWait

	busyDelta := currBusy - prevBusy
	idleDelta := currIdle - prevIdle
	totalDelta := busyDelta + idleDelta

	c.prevStats = current

	if totalDelta == 0 {
		return 0
	}

	// Usage = busy time / total time
	usage := float64(busyDelta) / float64(totalDelta) * 100
	return usage
}

// readCPUStats reads the aggregate CPU line from /proc/stat.
// The first line "cpu ..." contains combined stats for all CPUs.
// Format: cpu user nice system idle iowait irq softirq [steal guest guest_nice]
func (c *CPUMonitor) readCPUStats() *CPUStats {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// "cpu " (with space) is aggregate; "cpu0", "cpu1" etc are per-core
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) >= 8 {
				stats := &CPUStats{}
				stats.User, _ = strconv.ParseUint(fields[1], 10, 64)
				stats.Nice, _ = strconv.ParseUint(fields[2], 10, 64)
				stats.System, _ = strconv.ParseUint(fields[3], 10, 64)
				stats.Idle, _ = strconv.ParseUint(fields[4], 10, 64)
				stats.IOWait, _ = strconv.ParseUint(fields[5], 10, 64)
				stats.IRQ, _ = strconv.ParseUint(fields[6], 10, 64)
				stats.SoftIRQ, _ = strconv.ParseUint(fields[7], 10, 64)
				return stats
			}
		}
	}
	return nil
}

// MemoryInfo holds system memory statistics.
type MemoryInfo struct {
	TotalGB     float64 // Total physical RAM in gigabytes
	UsedGB      float64 // Used RAM in gigabytes (Total - Available)
	UsedPercent float64 // Usage as percentage (0-100)
}

// GetMemoryInfo reads memory statistics from /proc/meminfo.
// Uses MemAvailable (not MemFree) for accurate "used" calculation,
// as MemAvailable accounts for buffers/cache that can be reclaimed.
func GetMemoryInfo() *MemoryInfo {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil
	}
	defer file.Close()

	var memTotal, memAvailable uint64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseUint(fields[1], 10, 64)

		switch fields[0] {
		case "MemTotal:":
			memTotal = value
		case "MemAvailable:":
			memAvailable = value
		}
	}

	if memTotal == 0 {
		return nil
	}

	memUsed := memTotal - memAvailable
	return &MemoryInfo{
		TotalGB:     float64(memTotal) / 1024 / 1024,
		UsedGB:      float64(memUsed) / 1024 / 1024,
		UsedPercent: float64(memUsed) / float64(memTotal) * 100,
	}
}

// ZFSPool holds information about a ZFS storage pool.
type ZFSPool struct {
	Name        string  // Pool name (e.g., "fastpool", "tank")
	Size        string  // Total size, human readable (e.g., "2T", "500G")
	Used        string  // Used space, human readable
	Free        string  // Free space, human readable
	UsedPercent float64 // Capacity percentage (0-100)
}

// GetZFSPools queries all ZFS pools using the zpool command.
// Runs: zpool list -H -o name,size,alloc,free,cap
// Has a 2-second timeout to prevent hanging if ZFS is unresponsive.
// Returns nil if zpool command fails or no pools exist.
func GetZFSPools() []ZFSPool {
	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// -H: scripted mode (no headers, tab-separated)
	// -o: select specific columns
	cmd := exec.CommandContext(ctx, "zpool", "list", "-H", "-o", "name,size,alloc,free,cap")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var pools []ZFSPool
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			// Capacity field comes as "45%" - strip the percent sign
			capStr := strings.TrimSuffix(fields[4], "%")
			cap, _ := strconv.ParseFloat(capStr, 64)

			pools = append(pools, ZFSPool{
				Name:        fields[0],
				Size:        fields[1],
				Used:        fields[2],
				Free:        fields[3],
				UsedPercent: cap,
			})
		}
	}

	return pools
}

// GetLinkSpeed returns the link speed of a network interface in Mbps.
// Reads from /sys/class/net/<iface>/speed.
// Returns 0 if the speed cannot be determined (e.g., no link).
func GetLinkSpeed(iface string) int {
	path := "/sys/class/net/" + iface + "/speed"
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	speed, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || speed < 0 {
		return 0
	}

	return speed
}

// FormatLinkSpeed converts link speed in Mbps to human-readable format.
// Examples: 1000 -> "1G", 2500 -> "2.5G", 10000 -> "10G", 100 -> "100M"
func FormatLinkSpeed(speedMbps int) string {
	if speedMbps >= 1000 {
		gbps := float64(speedMbps) / 1000
		if gbps == float64(int(gbps)) {
			return strconv.Itoa(int(gbps)) + "G"
		}
		return strconv.FormatFloat(gbps, 'f', 1, 64) + "G"
	}
	return strconv.Itoa(speedMbps) + "M"
}

// GetUptime returns system uptime formatted as "Xd Yh" or "Xh Ym".
// Reads from /proc/uptime which contains uptime in seconds.
func GetUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "N/A"
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return "N/A"
	}

	uptimeSec, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "N/A"
	}

	return FormatDuration(int64(uptimeSec))
}

// FormatDuration formats seconds into human-readable duration.
// Format: "Xd Yh" for days+hours, "Xh Ym" for hours+minutes.
func FormatDuration(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	if days > 0 {
		return strconv.FormatInt(days, 10) + "d " + strconv.FormatInt(hours, 10) + "h"
	}
	if hours > 0 {
		return strconv.FormatInt(hours, 10) + "h " + strconv.FormatInt(minutes, 10) + "m"
	}
	return strconv.FormatInt(minutes, 10) + "m"
}

// GetLoggedInUsers returns a list of currently logged-in users by reading
// /var/run/utmp directly, avoiding a process spawn every call.
// On Linux x86_64, each utmp record is 384 bytes; ut_type (int16) is at
// offset 0 and ut_user (char[32]) is at offset 44. USER_PROCESS = 7.
// Returns empty slice if no users are logged in or on error.
func GetLoggedInUsers() []string {
	const (
		utmpPath    = "/var/run/utmp"
		recordSize  = 384
		userProcess = 7
		userOffset  = 44
		userLen     = 32
	)

	data, err := os.ReadFile(utmpPath)
	if err != nil {
		return nil
	}

	var users []string
	seen := make(map[string]bool)

	for i := 0; i+recordSize <= len(data); i += recordSize {
		rec := data[i : i+recordSize]

		utType := int16(rec[0]) | int16(rec[1])<<8
		if utType != userProcess {
			continue
		}

		raw := rec[userOffset : userOffset+userLen]
		n := 0
		for n < len(raw) && raw[n] != 0 {
			n++
		}
		user := string(raw[:n])

		if user != "" && !seen[user] {
			seen[user] = true
			users = append(users, user)
		}
	}

	return users
}

// ZFSStatus holds the current status of ZFS maintenance operations and health.
type ZFSStatus struct {
	ScrubInProgress    bool     // True if any pool is being scrubbed
	ResilverInProgress bool     // True if any pool is resilvering (rebuilding)
	DegradedPools      []string // Names of pools in DEGRADED state
	ErrorCount         int      // Total read/write/checksum errors across all pools
	ActivePool         string   // Name of pool with active operation (for display)
	ScanProgress       string   // Progress string (e.g., "45.2%")
}

// GetZFSStatus checks the status of all ZFS pools for maintenance operations and health.
// Parses 'zpool status' output to detect scrubs, resilvers, degraded pools, and errors.
// Returns nil if zpool command fails or ZFS is not available.
func GetZFSStatus() *ZFSStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "zpool", "status")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	status := &ZFSStatus{}
	lines := strings.Split(string(output), "\n")

	var currentPool string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect pool name (lines starting with "pool: ")
		if strings.HasPrefix(trimmed, "pool:") {
			currentPool = strings.TrimSpace(strings.TrimPrefix(trimmed, "pool:"))
		}

		// Detect pool state
		if strings.HasPrefix(trimmed, "state:") {
			state := strings.TrimSpace(strings.TrimPrefix(trimmed, "state:"))
			if state == "DEGRADED" || state == "FAULTED" || state == "OFFLINE" {
				status.DegradedPools = append(status.DegradedPools, currentPool)
			}
		}

		// Detect scan status (scrub/resilver in progress)
		if strings.HasPrefix(trimmed, "scan:") {
			scanLine := strings.ToLower(trimmed)
			if strings.Contains(scanLine, "scrub in progress") {
				status.ScrubInProgress = true
				status.ActivePool = currentPool
				// Try to extract progress percentage
				if idx := strings.Index(trimmed, "done"); idx > 0 {
					// Look for percentage before "done"
					parts := strings.Fields(trimmed[:idx])
					for _, p := range parts {
						if strings.HasSuffix(p, "%") {
							status.ScanProgress = p
							break
						}
					}
				}
			}
			if strings.Contains(scanLine, "resilver in progress") {
				status.ResilverInProgress = true
				status.ActivePool = currentPool
				if idx := strings.Index(trimmed, "done"); idx > 0 {
					parts := strings.Fields(trimmed[:idx])
					for _, p := range parts {
						if strings.HasSuffix(p, "%") {
							status.ScanProgress = p
							break
						}
					}
				}
			}
		}

		// Detect errors in device lines (READ WRITE CKSUM columns)
		// Device lines have format: NAME STATE READ WRITE CKSUM
		// Skip header and summary lines
		if strings.Contains(line, "ONLINE") || strings.Contains(line, "DEGRADED") ||
			strings.Contains(line, "FAULTED") || strings.Contains(line, "OFFLINE") {
			fields := strings.Fields(trimmed)
			// Device lines typically have 5 fields: name state read write cksum
			if len(fields) >= 5 {
				// Parse error counts (last 3 fields)
				read, _ := strconv.Atoi(fields[len(fields)-3])
				write, _ := strconv.Atoi(fields[len(fields)-2])
				cksum, _ := strconv.Atoi(fields[len(fields)-1])
				status.ErrorCount += read + write + cksum
			}
		}
	}

	return status
}
