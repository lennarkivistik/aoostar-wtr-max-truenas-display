package dashboard

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"go-wtr-display/network"
	"go-wtr-display/wave"
)

// DemoState is deterministic and uses no hardware, subprocesses or live data.
func DemoState(now time.Time, history time.Duration) *State {
	cpuTemp := 42.0
	s := &State{TrueNASVersion: "25.10.3.1", IPAddress: "192.0.2.10", Interface: "eth0", LinkSpeed: 2500, LinkState: "up", Uptime: "12d 4h",
		DisplayRxSpeed: 18_400_000, DisplayTxSpeed: 4_600_000, DisplayCPU: 12, CPUAvailable: true, NetworkAvailable: true,
		Memory: &network.MemoryInfo{TotalGB: 64, AvailableGB: 24, UsedPercent: 62.5}, ARC: &network.ARCInfo{SizeGB: 32, TargetGB: 40},
		Pools:        []network.ZFSPool{{Name: "archive", Used: "8T", Size: "32T", Free: "24T", UsedPercent: 25}, {Name: "flash", Used: "400G", Size: "2T", Free: "1.6T", UsedPercent: 20}},
		ZFSStatus:    &network.ZFSStatus{PoolStates: map[string]string{"archive": "ONLINE", "flash": "ONLINE", "boot-pool": "ONLINE"}},
		Temperatures: network.Temperatures{CPU: &cpuTemp, Disks: map[string]float64{}},
		WaveAnimator: wave.NewAnimatorWithHistory(history),
	}
	for _, kind := range []string{"sata", "nvme"} {
		count := 6
		if kind == "nvme" {
			count = 5
		}
		for bay := 1; bay <= count; bay++ {
			device := fmt.Sprintf("%s%d", kind, bay)
			s.Disks = append(s.Disks, network.DiskSlot{Bay: bay, Kind: kind, Device: device, Present: kind == "nvme" || bay <= 4})
			s.Temperatures.Disks[device] = float64(32 + bay)
		}
	}
	seconds := int(math.Ceil(history.Seconds()))
	for i := 0; i <= seconds+1; i++ {
		rx := 18e6 + 14e6*math.Sin(float64(i)*0.54) + 3e6*math.Sin(float64(i)*1.8)
		tx := 6e6 + 5e6*math.Sin(float64(i)*0.62+1.5)
		s.WaveAnimator.AddSampleAt(rx, tx, now.Add(time.Duration(i-seconds-1)*time.Second))
	}
	return s
}

func WritePreviews(dir string, o Options) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	now := time.Unix(1000, 0)
	for _, view := range []View{ViewNetwork, ViewStorage, ViewSystem, ViewDrives} {
		s := DemoState(now, o.WaveHistory)
		if err := writePreview(filepath.Join(dir, string(view)+".png"), s, Page{View: view}, o, now); err != nil {
			return err
		}
	}
	fault := DemoState(now, o.WaveHistory)
	fault.ZFSStatus.PoolStates["archive"] = "DEGRADED"
	fault.ZFSStatus.DegradedPools = []string{"archive"}
	if err := writePreview(filepath.Join(dir, "fault.png"), fault, Page{View: ViewNetwork}, o, now); err != nil {
		return err
	}
	unknown := &State{Interface: "eth0", LinkState: "unknown"}
	return writePreview(filepath.Join(dir, "unavailable.png"), unknown, Page{View: ViewSystem}, o, now)
}

func writePreview(path string, s *State, p Page, o Options, now time.Time) error {
	r := NewFrameRenderer()
	img, _ := r.Frame(s, p, o, now, true)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, img)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
