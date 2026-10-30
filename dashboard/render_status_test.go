package dashboard

import (
	"go-wtr-display/network"
	"testing"
	"time"
)

func TestFaultCannotBeHiddenByInfoOrMaintenance(t *testing.T) {
	s := DemoState(time.Unix(1000, 0), 20*time.Second)
	s.ZFSStatus.DegradedPools = []string{"tank"}
	s.ZFSStatus.PoolStates["tank"] = "DEGRADED"
	s.ZFSStatus.ResilverInProgress = true
	s.SetActiveNotification(&Notification{Message: "Backup complete", Theme: ThemeInfo})
	h := CurrentHealth(s)
	if !h.Critical || h.Title != "POOL FAULT" {
		t.Fatalf("fault lost: %+v", h)
	}
}

func TestUnknownIsNotHealthy(t *testing.T) {
	s := DemoState(time.Unix(1000, 0), 20*time.Second)
	s.ZFSStatus = nil
	if h := CurrentHealth(s); !h.Warning || h.Title != "CHECK DATA" {
		t.Fatalf("missing data healthy: %+v", h)
	}
	s.ZFSStatus = &network.ZFSStatus{}
	s.ZFSReadFailed = true
	if h := CurrentHealth(s); !h.Warning {
		t.Fatalf("failed refresh healthy: %+v", h)
	}
}

func TestCapacityChecksAllPools(t *testing.T) {
	s := DemoState(time.Unix(1000, 0), 20*time.Second)
	s.Pools = append(s.Pools, network.ZFSPool{Name: "third", UsedPercent: 96})
	if h := CurrentHealth(s); !h.Critical || h.Title != "STORAGE FULL" {
		t.Fatalf("third pool fault missed: %+v", h)
	}
}

func TestLinkSpeedUnknownDoesNotMeanLinkDown(t *testing.T) {
	s := DemoState(time.Unix(1000, 0), 20*time.Second)
	s.LinkSpeed = 0
	if h := CurrentHealth(s); h.Critical {
		t.Fatalf("unknown speed treated as down: %+v", h)
	}
	s.LinkState = "down"
	if h := CurrentHealth(s); !h.Critical || h.Title != "NO NETWORK LINK" {
		t.Fatalf("down link missed: %+v", h)
	}
}
