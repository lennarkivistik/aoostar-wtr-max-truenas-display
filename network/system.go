// Package network system.go provides system resource monitoring:
// CPU usage, memory usage, and ZFS pool statistics.
package network

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
		return math.NaN()
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

	if currBusy < prevBusy || currIdle < prevIdle {
		c.prevStats = current
		return 0
	}
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
	AvailableGB float64
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

	if scanner.Err() != nil || memTotal == 0 || memAvailable > memTotal {
		return nil
	}

	memUsed := memTotal - memAvailable
	return &MemoryInfo{
		AvailableGB: float64(memAvailable) / 1024 / 1024,
		TotalGB:     float64(memTotal) / 1024 / 1024,
		UsedGB:      float64(memUsed) / 1024 / 1024,
		UsedPercent: float64(memUsed) / float64(memTotal) * 100,
	}
}

// ZFSPool holds usable root-dataset capacity for a ZFS pool.
type ZFSPool struct {
	Name        string  // Pool name (e.g., "fastpool", "tank")
	Size        string  // Usable total dataset capacity, human readable (used + available)
	Used        string  // Usable dataset used space, human readable
	Free        string  // Usable dataset available space, human readable
	UsedPercent float64 // Usable dataset capacity percentage (0-100)
}

// GetZFSPools queries usable root-dataset capacity using the zfs command.
// Runs: zfs list -H -p -o name,used,avail -d 0 -t filesystem
// Has a 2-second timeout to prevent hanging if ZFS is unresponsive.
// Returns nil if zfs command fails or no pools exist.
func GetZFSPools() []ZFSPool {
	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// -H: scripted mode (no headers, tab-separated)
	// -p: exact byte values for stable parsing
	// -o: select specific columns
	cmd := exec.CommandContext(ctx, "zfs", "list", "-H", "-p", "-o", "name,used,avail", "-d", "0", "-t", "filesystem")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseZFSPools(output)
}

func parseZFSPools(output []byte) []ZFSPool {
	var pools []ZFSPool
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if strings.Contains(fields[0], "/") {
			continue
		}
		used, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		avail, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			continue
		}

		total := used + avail
		usedPercent := 0.0
		if total > 0 {
			usedPercent = float64(used) / float64(total) * 100
		}

		pools = append(pools, ZFSPool{
			Name:        fields[0],
			Size:        formatZFSBytes(total),
			Used:        formatZFSBytes(used),
			Free:        formatZFSBytes(avail),
			UsedPercent: usedPercent,
		})
	}

	return pools
}

func formatZFSBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatUint(bytes, 10) + "B"
	}

	value := float64(bytes)
	suffixes := []string{"B", "K", "M", "G", "T", "P", "E"}
	suffix := 0
	for value >= unit && suffix < len(suffixes)-1 {
		value /= unit
		suffix++
	}

	if value >= 10 && math.Abs(value-math.Round(value)) < 0.05 {
		return strconv.FormatFloat(value, 'f', 0, 64) + suffixes[suffix]
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + suffixes[suffix]
}

// DiskSlot describes one physical AOOSTAR WTR MAX drive bay.
type DiskSlot struct {
	Bay       int
	Kind      string
	Path      string
	Present   bool
	Device    string
	Model     string
	Serial    string
	Size      string
	Transport string
}

type bayMapping struct {
	bay  int
	kind string
	path string
}

var aoostarBayMappings = []bayMapping{
	{bay: 1, kind: "sata", path: "pci-0000:67:00.0-ata-6.0"},
	{bay: 2, kind: "sata", path: "pci-0000:67:00.0-ata-5.0"},
	{bay: 3, kind: "sata", path: "pci-0000:67:00.0-ata-4.0"},
	{bay: 4, kind: "sata", path: "pci-0000:67:00.0-ata-3.0"},
	{bay: 5, kind: "sata", path: "pci-0000:67:00.0-ata-2.0"},
	{bay: 6, kind: "sata", path: "pci-0000:67:00.0-ata-1.0"},
	{bay: 1, kind: "nvme", path: "pci-0000:6a:00.0-nvme-1"},
	{bay: 2, kind: "nvme", path: "pci-0000:6b:00.0-nvme-1"},
	{bay: 3, kind: "nvme", path: "pci-0000:6c:00.0-nvme-1"},
	{bay: 4, kind: "nvme", path: "pci-0000:6d:00.0-nvme-1"},
	{bay: 5, kind: "nvme", path: "pci-0000:68:00.0-nvme-1"},
}

var ignoredDiskByPaths = map[string]bool{
	"pci-0000:03:00.4-usbv3-0:1.4:1.0-scsi-0:0:0:0": true,
	"pci-0000:03:00.4-usb-0:1.4:1.0-scsi-0:0:0:0":   true,
}

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name      string        `json:"name"`
	Type      string        `json:"type"`
	Model     string        `json:"model"`
	Serial    string        `json:"serial"`
	Size      string        `json:"size"`
	Transport string        `json:"tran"`
	Path      string        `json:"path"`
	Children  []lsblkDevice `json:"children"`
}

// GetDiskSlots returns physical slot status using stable /dev/disk/by-path
// links. Kernel sdX names are only used as labels because they can reorder.
func GetDiskSlots() []DiskSlot {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lsblk", "-J", "-o", "NAME,TYPE,MODEL,SERIAL,SIZE,TRAN,PATH")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	paths := readDiskByPathTargets()
	if len(paths) == 0 {
		return nil
	}
	slots, err := ParseDiskSlots(output, paths)
	if err != nil {
		return nil
	}
	return slots
}

// ParseDiskSlots maps lsblk JSON and by-path symlinks onto fixed physical slots.
func ParseDiskSlots(lsblkJSON []byte, pathTargets map[string]string) ([]DiskSlot, error) {
	var parsed lsblkOutput
	if err := json.Unmarshal(lsblkJSON, &parsed); err != nil {
		return nil, err
	}

	disksByName := make(map[string]lsblkDevice)
	collectDiskDevices(parsed.BlockDevices, disksByName)

	slots := defaultDiskSlots()
	for i := range slots {
		slot := &slots[i]

		var disk lsblkDevice
		if devName := pathTargets[slot.Path]; devName != "" {
			disk = disksByName[devName]
		}
		if disk.Name == "" {
			continue
		}

		slot.Present = true
		slot.Device = disk.Name
		slot.Model = strings.TrimSpace(disk.Model)
		slot.Serial = strings.TrimSpace(disk.Serial)
		slot.Size = strings.TrimSpace(disk.Size)
		slot.Transport = strings.TrimSpace(disk.Transport)
	}

	return slots, nil
}

func defaultDiskSlots() []DiskSlot {
	slots := make([]DiskSlot, 0, len(aoostarBayMappings))
	for _, mapping := range aoostarBayMappings {
		slots = append(slots, DiskSlot{
			Bay:  mapping.bay,
			Kind: mapping.kind,
			Path: mapping.path,
		})
	}
	return slots
}

func collectDiskDevices(devices []lsblkDevice, byName map[string]lsblkDevice) {
	for _, dev := range devices {
		if dev.Type == "disk" {
			byName[dev.Name] = dev
		}
		collectDiskDevices(dev.Children, byName)
	}
}

func readDiskByPathTargets() map[string]string {
	paths, err := filepath.Glob("/dev/disk/by-path/*")
	if err != nil {
		return nil
	}
	sort.Strings(paths)

	targets := make(map[string]string)
	for _, path := range paths {
		name := filepath.Base(path)
		if strings.Contains(name, "-part") {
			continue
		}
		if ignoredDiskByPaths[name] {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		targets[name] = filepath.Base(target)
	}
	return targets
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
	PoolStates         map[string]string // Explicit health per pool; absent means unknown.
	ScrubInProgress    bool              // True if any pool is being scrubbed
	ResilverInProgress bool              // True if any pool is resilvering (rebuilding)
	DegradedPools      []string          // Names of pools in DEGRADED state
	ErrorCount         int               // Total read/write/checksum errors across all pools
	ActivePool         string            // Name of pool with active operation (for display)
	ScanProgress       string            // Progress string (e.g., "45.2%")
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

	return parseZFSStatus(output)
}

func parseZFSStatus(output []byte) *ZFSStatus {
	status := &ZFSStatus{PoolStates: make(map[string]string)}
	var currentPool string
	activeScan := false
	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		switch {
		case strings.HasPrefix(trimmed, "pool:"):
			currentPool = strings.TrimSpace(strings.TrimPrefix(trimmed, "pool:"))
			activeScan = false
		case strings.HasPrefix(trimmed, "state:"):
			health := strings.TrimSpace(strings.TrimPrefix(trimmed, "state:"))
			status.PoolStates[currentPool] = health
			if health != "ONLINE" {
				status.DegradedPools = append(status.DegradedPools, currentPool)
			}
		case strings.HasPrefix(trimmed, "scan:"):
			activeScan = false
			if strings.Contains(trimmed, "in progress") {
				resilver := strings.Contains(trimmed, "resilver")
				scrub := strings.Contains(trimmed, "scrub")
				activeScan = (resilver && !status.ResilverInProgress) ||
					(scrub && !status.ResilverInProgress && !status.ScrubInProgress)
				status.ResilverInProgress = status.ResilverInProgress || resilver
				status.ScrubInProgress = status.ScrubInProgress || scrub
				if activeScan {
					status.ActivePool = currentPool
					status.ScanProgress = ""
				}
			}
		case strings.HasPrefix(trimmed, "config:"):
			activeScan = false
		}
		if activeScan {
			for _, field := range fields {
				if strings.HasSuffix(field, "%") {
					status.ScanProgress = field
				}
			}
		}
		// Count leaf devices, not aggregate pool/vdev rows which duplicate errors.
		if len(fields) == 5 && fields[0] != currentPool && !isVdev(fields[0]) {
			switch fields[1] {
			case "ONLINE", "DEGRADED", "FAULTED", "OFFLINE", "UNAVAIL", "REMOVED":
				for _, field := range fields[2:] {
					status.ErrorCount += parseErrorCount(field)
				}
			}
		}
		if strings.HasPrefix(trimmed, "errors:") && !strings.Contains(trimmed, "No known data errors") {
			// Permanent data errors may be reported independently of device counters.
			if status.ErrorCount == 0 && trimmed != "errors: No known data errors" {
				status.ErrorCount = 1
			}
		}
	}
	if len(status.PoolStates) == 0 {
		return nil
	}
	return status
}

func isVdev(name string) bool {
	for _, prefix := range []string{"mirror-", "raidz", "draid", "replacing-", "spare-"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func parseErrorCount(s string) int {
	multiplier := 1.0
	if len(s) > 0 {
		switch s[len(s)-1] {
		case 'K':
			multiplier = 1e3
			s = s[:len(s)-1]
		case 'M':
			multiplier = 1e6
			s = s[:len(s)-1]
		case 'G':
			multiplier = 1e9
			s = s[:len(s)-1]
		}
	}
	value, err := strconv.ParseFloat(s, 64)
	if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int(math.Min(value*multiplier, 1e12))
}
