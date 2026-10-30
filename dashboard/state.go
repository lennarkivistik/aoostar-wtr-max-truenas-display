package dashboard

import (
	"image"
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
	LinkState      string
	LinkSpeed      int
	Uptime         string

	// Displayed values (short exponential average, sampled once per second)
	DisplayRxSpeed   float64
	DisplayTxSpeed   float64
	DisplayCPU       float64
	CPUAvailable     bool
	NetworkAvailable bool
	ARC              *network.ARCInfo
	Temperatures     network.Temperatures

	// Hysteresis tracking for speed unit display (prevents flicker at boundaries)
	LastRxUnit string
	LastTxUnit string

	// Slow-updating data (every 5s)
	Memory *network.MemoryInfo
	Pools  []network.ZFSPool
	Disks  []network.DiskSlot

	// Wave animation (packets that flow across the screen)
	WaveAnimator *wave.Animator

	// Notification system
	activeNotification   *Notification // Current notification to display (nil = none)
	NotificationsChanged chan struct{} // Coalesced wakeups for prompt notifications.
	notificationMutex    sync.RWMutex  // protects activeNotification across socket goroutines
	LoggedInUsers        []string      // Current shell sessions
	ZFSReadFailed        bool
	PoolReadFailed       bool
	ZFSStatus            *network.ZFSStatus // Current ZFS maintenance/health status

	// Screenshot support
	lastFrame  *image.RGBA // Last rendered frame (for screenshots)
	frameMutex sync.RWMutex
}

// NewState creates a new dashboard state.
func NewState(iface string, animator *wave.Animator) *State {
	return &State{
		NotificationsChanged: make(chan struct{}, 1),
		Interface:            iface,
		TrueNASVersion:       network.GetTrueNASVersion(),
		WaveAnimator:         animator,
	}
}

// SetActiveNotification replaces the active notification safely from any goroutine.
func (s *State) SetActiveNotification(notif *Notification) {
	s.notificationMutex.Lock()
	defer s.notificationMutex.Unlock()
	defer s.signalNotification()
	if notif == nil {
		s.activeNotification = nil
		return
	}
	cp := *notif
	s.activeNotification = &cp
}

// GetActiveNotification returns a copy of the current notification.
func (s *State) GetActiveNotification() *Notification {
	s.notificationMutex.RLock()
	defer s.notificationMutex.RUnlock()
	if s.activeNotification == nil {
		return nil
	}
	cp := *s.activeNotification
	return &cp
}

// ClearExpiredNotification removes the active notification if it has expired.
func (s *State) ClearExpiredNotification() {
	s.notificationMutex.Lock()
	defer s.notificationMutex.Unlock()
	if s.activeNotification != nil && s.activeNotification.IsExpired() {
		s.activeNotification = nil
		s.signalNotification()
	}
}

func (s *State) signalNotification() {
	select {
	case s.NotificationsChanged <- struct{}{}:
	default:
	}
}

// SetLastFrame stores the last completed frame for screenshot capture.
// A reusable owned buffer keeps screenshots safe even after failed transfers.
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
