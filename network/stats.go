// Package network provides monitoring for network interfaces, system resources,
// and TrueNAS-specific information like ZFS pools.
package network

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Stats holds raw network interface byte counters at a point in time.
// These counters are cumulative since boot and only increase.
type Stats struct {
	RxBytes uint64    // Total bytes received
	TxBytes uint64    // Total bytes transmitted
	Time    time.Time // Timestamp when stats were read
}

// Throughput holds calculated network throughput in bytes per second.
// Calculated by comparing two Stats samples over time.
type Throughput struct {
	RxBytesPerSec float64 // Download speed in bytes/second
	TxBytesPerSec float64 // Upload speed in bytes/second
}

// Monitor tracks network interface statistics over time to calculate throughput.
// It stores the previous reading to compute deltas between samples.
type Monitor struct {
	iface     string // Interface name (e.g., "eth0")
	prevStats *Stats // Previous sample for delta calculation
}

// NewMonitor creates a new network monitor for the specified interface.
// Call GetThroughput() repeatedly to get current speeds.
func NewMonitor(iface string) *Monitor {
	return &Monitor{
		iface: iface,
	}
}

// ListInterfaces returns a list of available network interface names.
// Reads from /sys/class/net which contains a directory for each interface.
// This is a Linux-specific implementation.
func ListInterfaces() ([]string, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, fmt.Errorf("failed to read /sys/class/net: %w", err)
	}

	var interfaces []string
	for _, entry := range entries {
		interfaces = append(interfaces, entry.Name())
	}
	return interfaces, nil
}

// GetStats reads current byte counters from /sys/class/net/<iface>/statistics/.
// Returns raw cumulative counters - use GetThroughput() for rate calculations.
func (m *Monitor) GetStats() (*Stats, error) {
	rxBytes, err := m.readStat("rx_bytes")
	if err != nil {
		return nil, err
	}

	txBytes, err := m.readStat("tx_bytes")
	if err != nil {
		return nil, err
	}

	return &Stats{
		RxBytes: rxBytes,
		TxBytes: txBytes,
		Time:    time.Now(),
	}, nil
}

// GetThroughput calculates network throughput since the last call.
// On first call, returns zero values and stores baseline.
// Subsequent calls return bytes/second based on delta from previous sample.
// Call this at regular intervals (e.g., every second) for accurate readings.
func (m *Monitor) GetThroughput() (*Throughput, error) {
	current, err := m.GetStats()
	if err != nil {
		return nil, err
	}

	throughput := throughputBetween(current, m.prevStats)
	m.prevStats = current
	return throughput, nil
}

func throughputBetween(current, previous *Stats) *Throughput {
	if previous == nil || current.RxBytes < previous.RxBytes || current.TxBytes < previous.TxBytes {
		return &Throughput{}
	}
	elapsed := current.Time.Sub(previous.Time).Seconds()
	if elapsed <= 0 {
		return &Throughput{}
	}
	return &Throughput{RxBytesPerSec: float64(current.RxBytes-previous.RxBytes) / elapsed, TxBytesPerSec: float64(current.TxBytes-previous.TxBytes) / elapsed}
}

func (m *Monitor) readStat(stat string) (uint64, error) {
	path := filepath.Join("/sys/class/net", m.iface, "statistics", stat)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s for %s: %w", stat, m.iface, err)
	}

	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s: %w", stat, err)
	}

	return value, nil
}

// GetInterfaceIP returns the IPv4 address assigned to the specified interface.
// Uses Go's net package to query interface addresses.
// Returns the first IPv4 address found; ignores IPv6.
func GetInterfaceIP(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("failed to get addresses for %s: %w", ifaceName, err)
	}

	for _, addr := range addrs {
		// Check if it's an IP network address
		if ipNet, ok := addr.(*net.IPNet); ok {
			// Get IPv4 address (skip IPv6)
			if ipv4 := ipNet.IP.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no IPv4 address found for %s", ifaceName)
}

// GetTrueNASVersion attempts to read the TrueNAS/system version.
// Checks multiple possible locations in order:
//   - /etc/version (TrueNAS primary)
//   - /etc/truenas_version
//   - /etc/ix-release
//   - /etc/os-release VERSION= field (fallback for other Linux)
//
// Returns "Unknown" if no version can be determined.
func GetTrueNASVersion() string {
	// Try common TrueNAS version file locations
	versionFiles := []string{
		"/etc/version",
		"/etc/truenas_version",
		"/etc/ix-release",
	}

	for _, path := range versionFiles {
		data, err := os.ReadFile(path)
		if err == nil {
			version := strings.TrimSpace(string(data))
			// Clean up the version string
			if version != "" {
				// Some files might have extra info, get first line
				lines := strings.Split(version, "\n")
				if len(lines) > 0 {
					return lines[0]
				}
				return version
			}
		}
	}

	// Try to get from /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "VERSION=") {
				version := strings.TrimPrefix(line, "VERSION=")
				version = strings.Trim(version, "\"")
				return version
			}
		}
	}

	return "Unknown"
}

// FormatSpeed converts bytes per second to human-readable format (KB/s, MB/s, GB/s).
// Uses binary units (1024-based) which is traditional for data transfer rates.
func FormatSpeed(bytesPerSec float64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytesPerSec >= GB:
		return fmt.Sprintf("%.1f GB/s", bytesPerSec/GB)
	case bytesPerSec >= MB:
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/MB)
	case bytesPerSec >= KB:
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/KB)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
}

// FormatBits converts bytes per second to bits per second for network display.
// Uses decimal units (1000-based) as is standard for network speeds.
// Output examples: "100 Mbps", "1.5 Gbps", "500 Kbps"
func FormatBits(bytesPerSec float64) string {
	bitsPerSec := bytesPerSec * 8
	const (
		Kbps = 1000
		Mbps = Kbps * 1000
		Gbps = Mbps * 1000
	)

	switch {
	case bitsPerSec >= Gbps:
		return fmt.Sprintf("%.2f Gbps", bitsPerSec/Gbps)
	case bitsPerSec >= Mbps:
		return fmt.Sprintf("%.1f Mbps", bitsPerSec/Mbps)
	case bitsPerSec >= Kbps:
		return fmt.Sprintf("%.0f Kbps", bitsPerSec/Kbps)
	default:
		return fmt.Sprintf("%.0f bps", bitsPerSec)
	}
}
