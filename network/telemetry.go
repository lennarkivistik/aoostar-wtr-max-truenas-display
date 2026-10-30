package network

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ARCInfo struct{ SizeGB, TargetGB float64 }

func GetARCInfo() *ARCInfo {
	data, err := os.ReadFile("/proc/spl/kstat/zfs/arcstats")
	if err != nil {
		return nil
	}
	return parseARC(data)
}

func parseARC(data []byte) *ARCInfo {
	values := map[string]float64{}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || (f[0] != "size" && f[0] != "c") {
			continue
		}
		if v, err := strconv.ParseUint(f[2], 10, 64); err == nil {
			values[f[0]] = float64(v) / (1024 * 1024 * 1024)
		}
	}
	size, ok := values["size"]
	if !ok {
		return nil
	}
	return &ARCInfo{SizeGB: size, TargetGB: values["c"]}
}

// LinkState distinguishes unavailable speed metadata from an actual down link.
func GetLinkState(iface string) string {
	data, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "operstate"))
	if err != nil {
		return "unknown"
	}
	switch strings.TrimSpace(string(data)) {
	case "up":
		return "up"
	case "down", "lowerlayerdown", "notpresent":
		return "down"
	default:
		return "unknown"
	}
}

type Temperatures struct {
	CPU   *float64
	Disks map[string]float64
}

// GetTemperatures uses kernel-exported sensors; it never invokes SMART commands.
func GetTemperatures(slots []DiskSlot) Temperatures {
	result := Temperatures{Disks: make(map[string]float64)}
	devices := make(map[string]string)
	for _, slot := range slots {
		if slot.Present {
			if path, err := filepath.EvalSymlinks("/sys/class/block/" + slot.Device + "/device"); err == nil {
				devices[slot.Device] = path
			}
		}
	}
	dirs, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, dir := range dirs {
		nameData, _ := os.ReadFile(filepath.Join(dir, "name"))
		name := strings.TrimSpace(string(nameData))
		sensorDevice, _ := filepath.EvalSymlinks(filepath.Join(dir, "device"))
		inputs, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		for _, input := range inputs {
			data, err := os.ReadFile(input)
			if err != nil {
				continue
			}
			milli, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			if err != nil || milli < -20000 || milli > 150000 {
				continue
			}
			temp := milli / 1000
			labelData, _ := os.ReadFile(strings.TrimSuffix(input, "_input") + "_label")
			label := strings.TrimSpace(string(labelData))
			if name == "k10temp" || name == "coretemp" {
				if label == "Tdie" || strings.HasPrefix(label, "Package id") || (result.CPU == nil && filepath.Base(input) == "temp1_input") {
					v := temp
					result.CPU = &v
				}
			}
			if name != "nvme" && name != "drivetemp" {
				continue
			}
			// temp1 is the composite sensor; avoid replacing it with NVMe internals.
			if filepath.Base(input) != "temp1_input" {
				continue
			}
			for device, path := range devices {
				if sensorDevice != "" && (path == sensorDevice || strings.HasPrefix(path, sensorDevice+"/")) {
					result.Disks[device] = temp
				}
			}
		}
	}
	return result
}
