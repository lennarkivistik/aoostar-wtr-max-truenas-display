package dashboard

import (
	"fmt"
	"strings"

	"go-wtr-display/network"
)

type Health struct {
	Title, Detail     string
	Critical, Warning bool
}

func visiblePools(pools []network.ZFSPool) []network.ZFSPool {
	var out []network.ZFSPool
	for _, p := range pools {
		if !strings.HasPrefix(p.Name, "boot") {
			out = append(out, p)
		}
	}
	return out
}

// CurrentHealth derives status from measured data; notifications never replace
// a critical condition. Unknown is deliberately separate from healthy.
func CurrentHealth(s *State) Health {
	if z := s.ZFSStatus; z != nil {
		if len(z.DegradedPools) > 0 {
			return Health{"POOL FAULT", z.DegradedPools[0] + " / " + z.PoolStates[z.DegradedPools[0]] + staleNote(s.ZFSReadFailed), true, false}
		}
		if z.ErrorCount > 0 {
			return Health{"ZFS ERRORS", "Check pool and device status" + staleNote(s.ZFSReadFailed), true, false}
		}
	}
	if s.LinkState == "down" {
		return Health{"NO NETWORK LINK", s.Interface, true, false}
	}
	pools := visiblePools(s.Pools)
	for _, p := range pools {
		if p.UsedPercent >= 95 {
			return Health{"STORAGE FULL", fmt.Sprintf("%s / %.0f%% used", p.Name, p.UsedPercent) + staleNote(s.PoolReadFailed), true, false}
		}
	}
	if n := s.GetActiveNotification(); n != nil && !n.IsExpired() && n.Theme == ThemeDanger {
		return Health{"ALERT", n.Message, true, false}
	}
	if z := s.ZFSStatus; z != nil {
		if z.ResilverInProgress {
			return Health{"RESILVERING", z.ActivePool + " " + z.ScanProgress + staleNote(s.ZFSReadFailed), false, true}
		}
		if z.ScrubInProgress {
			return Health{"SCRUBBING", z.ActivePool + " " + z.ScanProgress + staleNote(s.ZFSReadFailed), false, true}
		}
	}
	for _, p := range pools {
		if p.UsedPercent >= 90 {
			return Health{"CAPACITY LOW", fmt.Sprintf("%s / %.0f%% used", p.Name, p.UsedPercent), false, true}
		}
	}
	if !s.NetworkAvailable {
		return Health{"CHECK DATA", "Network readings unavailable", false, true}
	}
	if s.LinkState != "up" {
		return Health{"LINK UNKNOWN", "Link state unavailable", false, true}
	}
	if s.ZFSStatus == nil || s.ZFSReadFailed {
		return Health{"CHECK DATA", "ZFS health unavailable", false, true}
	}
	if s.Pools == nil || s.PoolReadFailed {
		return Health{"CHECK DATA", "Pool capacity unavailable", false, true}
	}
	if s.Disks == nil {
		return Health{"CHECK DATA", "Drive inventory unavailable", false, true}
	}
	if !s.CPUAvailable || s.Memory == nil {
		return Health{"CHECK DATA", "System readings unavailable", false, true}
	}
	return Health{Title: "POOLS ONLINE"}
}

func PoolNames(s *State) []string {
	var names []string
	for _, p := range visiblePools(s.Pools) {
		names = append(names, p.Name)
	}
	return names
}

func staleNote(failed bool) string {
	if failed {
		return " (last known)"
	}
	return ""
}
