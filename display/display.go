// Package display provides low-level communication with the WTR MAX / AOOSTAR
// front panel LCD display over USB serial.
//
// The display uses a proprietary protocol where images are sent as RGB565 data
// in 47-byte chunks. Commands use a simple header format starting with AA 55 AA 55.
package display

import (
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

const (
	// Width is the display resolution width in pixels
	Width = 960
	// Height is the display resolution height in pixels
	Height = 376

	// DefaultVID is the USB Vendor ID for the display's UART chip (Winbond)
	DefaultVID = 0x0416
	// DefaultPID is the USB Product ID for the display's UART chip
	DefaultPID = 0x90A1

	// BaudRate for serial communication (actual speed is USB bulk transfer)
	BaudRate = 1500000
)

// Protocol commands - all commands start with magic bytes AA 55 AA 55
var (
	// CmdDisplayOff turns the LCD backlight off
	CmdDisplayOff = []byte{0xAA, 0x55, 0xAA, 0x55, 0x0A, 0x00, 0x00, 0x00}
	// CmdDisplayOn turns the LCD backlight on, showing last displayed image
	CmdDisplayOn = []byte{0xAA, 0x55, 0xAA, 0x55, 0x0B, 0x00, 0x00, 0x00}
)

// Display represents a connection to the WTR MAX display.
// It handles serial port communication and implements the display protocol.
type Display struct {
	port serial.Port

	// Frame caching for differential updates
	enableCache bool   // Whether to use frame caching (default: true)
	prevFrame   []byte // Previous frame's RGB565 data for diff comparison

	rgb565Buffer []byte
	packetBuffer []byte
	dirtyChunks  []int
}

// Open opens a connection to the display using the specified device path.
// The device is typically /dev/ttyACM0 on Linux.
// It configures the serial port with 1.5Mbaud, 8N1 settings.
func Open(device string) (*Display, error) {
	mode := &serial.Mode{
		BaudRate: BaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(device, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", device, err)
	}

	// Set read timeout
	if err := port.SetReadTimeout(time.Second); err != nil {
		port.Close()
		return nil, fmt.Errorf("failed to set read timeout: %w", err)
	}

	return &Display{
		port:        port,
		enableCache: true, // Enable frame caching by default
	}, nil
}

// FindAndOpen attempts to auto-detect and open the display.
// It searches for the default USB VID:PID (0416:90A1).
func FindAndOpen() (*Display, error) {
	return FindAndOpenByID(DefaultVID, DefaultPID)
}

// FindAndOpenByID searches for a display with the specified USB VID:PID.
// It only opens USB serial ports whose enumerated VID/PID matches.
func FindAndOpenByID(vid, pid int) (*Display, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("failed to list detailed serial ports: %w", err)
	}

	wantVID := fmt.Sprintf("%04X", vid)
	wantPID := fmt.Sprintf("%04X", pid)
	var openErr error

	for _, port := range ports {
		if !matchesUSBVIDPID(port, wantVID, wantPID) {
			continue
		}
		d, err := Open(port.Name)
		if err == nil {
			return d, nil
		}
		openErr = err
	}

	if openErr != nil {
		return nil, fmt.Errorf("display with VID:PID %04x:%04x found but could not be opened: %w", vid, pid, openErr)
	}

	return nil, fmt.Errorf("display with VID:PID %04x:%04x not found", vid, pid)
}

func matchesUSBVIDPID(port *enumerator.PortDetails, wantVID, wantPID string) bool {
	return port != nil &&
		port.IsUSB &&
		strings.EqualFold(port.VID, wantVID) &&
		strings.EqualFold(port.PID, wantPID)
}

// Close closes the connection to the display
func (d *Display) Close() error {
	if d.port != nil {
		return d.port.Close()
	}
	return nil
}

// On turns the display on
func (d *Display) On() error {
	return d.sendAndDrain(CmdDisplayOn)
}

// Off turns the display off
func (d *Display) Off() error {
	return d.sendAndDrain(CmdDisplayOff)
}

// SetCacheEnabled enables or disables frame caching.
// When enabled, only changed chunks are sent to the display, dramatically
// improving update speed for partial screen changes (e.g., updating numbers).
// Enabled by default.
func (d *Display) SetCacheEnabled(enabled bool) {
	d.enableCache = enabled
	if !enabled {
		d.ClearCache()
	}
}

// ClearCache clears the cached frame, forcing a full redraw on next SendImage.
// Useful after display power cycle or to force a complete refresh.
func (d *Display) ClearCache() {
	d.prevFrame = nil
}

// send writes data to the display without waiting for completion.
// Used for bulk image transfers where we send many chunks quickly.
// The data is written to the serial port's output buffer.
func (d *Display) send(data []byte) error {
	n, err := d.port.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to display: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("incomplete write: wrote %d of %d bytes", n, len(data))
	}
	return nil
}

// sendAndDrain writes data and waits for all bytes to be transmitted.
// Used for commands (on/off) and end-of-image where we need confirmation.
// Includes retry logic for EINTR (interrupted system call) which can occur
// on busy systems.
func (d *Display) sendAndDrain(data []byte) error {
	if err := d.send(data); err != nil {
		return err
	}
	// Drain with retry for interrupted system calls (EINTR)
	for i := 0; i < 3; i++ {
		if err := d.port.Drain(); err != nil {
			if i < 2 {
				continue // retry on interrupt
			}
			return fmt.Errorf("failed to drain port: %w", err)
		}
		break
	}
	return nil
}
