package dashboard

import (
	"testing"
	"time"
)

func TestRefreshIntervalHonorsLoadVisibilityAndTransferCost(t *testing.T) {
	s := DemoState(time.Unix(1000, 0), 20*time.Second)
	o := DefaultOptions()
	if got := RefreshInterval(s, Page{View: ViewNetwork}, o, 0); got != 250*time.Millisecond {
		t.Fatalf("network interval: %v", got)
	}
	if got := RefreshInterval(s, Page{View: ViewStorage}, o, 0); got != time.Second {
		t.Fatalf("static interval: %v", got)
	}
	o.FrameInterval = 100 * time.Millisecond
	s.DisplayCPU = 80
	if got := RefreshInterval(s, Page{View: ViewNetwork}, o, 0); got < 2*time.Second {
		t.Fatalf("manual FPS bypassed load guard: %v", got)
	}
	s.DisplayCPU = 5
	if got := RefreshInterval(s, Page{View: ViewNetwork}, o, 400*time.Millisecond); got < 800*time.Millisecond {
		t.Fatalf("transfer budget ignored: %v", got)
	}
}

func TestNotificationAccessCopiesState(t *testing.T) {
	state := &State{}
	original := &Notification{Message: "Backup complete", Theme: ThemeInfo}
	state.SetActiveNotification(original)
	original.Message = "mutated"

	got := state.GetActiveNotification()
	if got == nil || got.Message != "Backup complete" {
		t.Fatalf("notification = %#v, want original copy", got)
	}

	got.Message = "client mutation"
	again := state.GetActiveNotification()
	if again == nil || again.Message != "Backup complete" {
		t.Fatalf("stored notification mutated through getter: %#v", again)
	}
}
