package dashboard

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"go-wtr-display/network"
	"go-wtr-display/wave"
)

// NotificationTheme defines the visual style of a notification
type NotificationTheme string

const (
	ThemeInfo    NotificationTheme = "info"    // White background, black text
	ThemeWarning NotificationTheme = "warning" // Yellow background, black text
	ThemeDanger  NotificationTheme = "danger"  // Red background, white text
)

// Notification represents a message to display on screen
type Notification struct {
	Message   string            // Text to display
	Theme     NotificationTheme // Visual theme
	ExpiresAt time.Time         // When to auto-dismiss (zero = persistent)
}

// IsExpired returns true if the notification should be dismissed
func (n *Notification) IsExpired() bool {
	if n.ExpiresAt.IsZero() {
		return false // Persistent notification
	}
	return time.Now().After(n.ExpiresAt)
}

// State holds all data for rendering the dashboard.
type State struct {
	// Static info (loaded once or periodically)
	TrueNASVersion string
	IPAddress      string
	Interface      string
	LinkSpeed      int
	Uptime         string

	// Displayed values (rolling average, updated every 1s for stable readability)
	DisplayRxSpeed float64
	DisplayTxSpeed float64
	DisplayCPU     float64

	// Hysteresis tracking for speed unit display (prevents flicker at boundaries)
	LastRxUnit string
	LastTxUnit string

	// For rolling average calculation
	RecentRx []float64
	RecentTx []float64

	// Slow-updating data (every 5s)
	Memory *network.MemoryInfo
	Pools  []network.ZFSPool

	// Wave animation (packets that flow across the screen)
	WaveAnimator *wave.Animator

	// Alerts
	Alerts []string

	// Notification system
	ActiveNotification *Notification      // Current notification to display (nil = none)
	LoggedInUsers      []string           // Currently logged-in users (for red dot indicator)
	ZFSStatus          *network.ZFSStatus // Current ZFS maintenance/health status

	// Screenshot support
	lastFrame  *image.RGBA // Last rendered frame (for screenshots)
	frameMutex sync.RWMutex
}

// NewState creates a new dashboard state.
func NewState(iface string, animator *wave.Animator) *State {
	return &State{
		Interface:      iface,
		TrueNASVersion: network.GetTrueNASVersion(),
		WaveAnimator:   animator,
		RecentRx:       make([]float64, 0, 10),
		RecentTx:       make([]float64, 0, 10),
	}
}

// CheckAlerts evaluates system state and returns any active alerts.
func (s *State) CheckAlerts() []string {
	var alerts []string

	// Check pool usage
	for _, pool := range s.Pools {
		if pool.UsedPercent >= 95 {
			alerts = append(alerts, fmt.Sprintf("%s pool CRITICAL: %.0f%%", pool.Name, pool.UsedPercent))
		} else if pool.UsedPercent >= 90 {
			alerts = append(alerts, fmt.Sprintf("%s pool > 90%%", pool.Name))
		}
	}

	// Check if link is down
	if s.LinkSpeed == 0 {
		alerts = append(alerts, "No network link")
	}

	return alerts
}

// Average calculates the average of a slice of float64.
func Average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// UpdateNotificationFromZFS updates the active notification based on ZFS status.
// Danger notifications (degraded pools, errors) take priority over warnings (maintenance).
func (s *State) UpdateNotificationFromZFS() {
	if s.ZFSStatus == nil {
		return
	}

	// Priority 1: Degraded pools (danger - persistent)
	if len(s.ZFSStatus.DegradedPools) > 0 {
		s.ActiveNotification = &Notification{
			Message: fmt.Sprintf("DEGRADED: %s", s.ZFSStatus.DegradedPools[0]),
			Theme:   ThemeDanger,
			// No expiry - persistent until resolved
		}
		return
	}

	// Priority 2: Disk errors (danger - persistent)
	if s.ZFSStatus.ErrorCount > 0 {
		s.ActiveNotification = &Notification{
			Message: fmt.Sprintf("Disk errors detected: %d", s.ZFSStatus.ErrorCount),
			Theme:   ThemeDanger,
		}
		return
	}

	// Priority 3: Resilver in progress (warning - persistent while active)
	if s.ZFSStatus.ResilverInProgress {
		msg := fmt.Sprintf("Resilver: %s", s.ZFSStatus.ActivePool)
		if s.ZFSStatus.ScanProgress != "" {
			msg += " " + s.ZFSStatus.ScanProgress
		}
		s.ActiveNotification = &Notification{
			Message: msg,
			Theme:   ThemeWarning,
		}
		return
	}

	// Priority 4: Scrub in progress (warning - persistent while active)
	if s.ZFSStatus.ScrubInProgress {
		msg := fmt.Sprintf("Scrub: %s", s.ZFSStatus.ActivePool)
		if s.ZFSStatus.ScanProgress != "" {
			msg += " " + s.ZFSStatus.ScanProgress
		}
		s.ActiveNotification = &Notification{
			Message: msg,
			Theme:   ThemeWarning,
		}
		return
	}

	// No ZFS issues - clear notification if it was ZFS-related
	// (Don't clear if it's a different type of notification)
	if s.ActiveNotification != nil {
		msg := s.ActiveNotification.Message
		if strings.Contains(msg, "Scrub") || strings.Contains(msg, "Resilver") ||
			strings.Contains(msg, "DEGRADED") || strings.Contains(msg, "errors") {
			s.ActiveNotification = nil
		}
	}
}

// ClearExpiredNotification removes the active notification if it has expired.
func (s *State) ClearExpiredNotification() {
	if s.ActiveNotification != nil && s.ActiveNotification.IsExpired() {
		s.ActiveNotification = nil
	}
}

// SetLastFrame stores a copy of the rendered frame for screenshot capture.
// Copies the pixel data so the caller can safely reuse the source buffer.
func (s *State) SetLastFrame(img *image.RGBA) {
	s.frameMutex.Lock()
	defer s.frameMutex.Unlock()
	if s.lastFrame == nil || s.lastFrame.Bounds() != img.Bounds() {
		s.lastFrame = image.NewRGBA(img.Bounds())
	}
	copy(s.lastFrame.Pix, img.Pix)
}

// GetLastFrame returns a copy of the last rendered frame.
// Returns nil if no frame has been rendered yet.
func (s *State) GetLastFrame() *image.RGBA {
	s.frameMutex.RLock()
	defer s.frameMutex.RUnlock()

	if s.lastFrame == nil {
		return nil
	}

	bounds := s.lastFrame.Bounds()
	cp := image.NewRGBA(bounds)
	copy(cp.Pix, s.lastFrame.Pix)
	return cp
}
