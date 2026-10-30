// Package dashboard notify.go provides Unix socket-based notification IPC.
// This allows external scripts to send notifications to the running display
// and request screenshots.
package dashboard

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net"
	"os"
	"syscall"
	"time"
	"unicode/utf8"
)

// DefaultSocketPath is the Unix socket path for notification IPC
const DefaultSocketPath = "/tmp/wtr-display.sock"

// SocketMessage is the JSON structure for incoming commands
type SocketMessage struct {
	Command  string `json:"command"`  // "notify" or "screenshot" (empty = notify for backwards compat)
	Message  string `json:"message"`  // For notify: the text to display
	Theme    string `json:"theme"`    // For notify: "info", "warning", "danger"
	Duration int    `json:"duration"` // For notify: seconds, 0 = persistent
}

// NotifyServer listens for incoming notifications on a Unix socket
type NotifyServer struct {
	listener net.Listener
	state    *State
	stopChan chan struct{}
	clients  chan struct{}
}

// NewNotifyServer creates a new notification server
func NewNotifyServer(state *State) (*NotifyServer, error) {
	// Never unlink a live instance's socket or an unrelated filesystem entry.
	if info, err := os.Lstat(DefaultSocketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("notification path is not a socket: %s", DefaultSocketPath)
		}
		conn, dialErr := net.DialTimeout("unix", DefaultSocketPath, 200*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			return nil, fmt.Errorf("display instance already running")
		}
		// A socket with denied access may still be live. Only connection refusal
		// is evidence that this pathname is stale.
		if !errors.Is(dialErr, syscall.ECONNREFUSED) {
			return nil, fmt.Errorf("cannot verify existing socket: %w", dialErr)
		}
		if err := os.Remove(DefaultSocketPath); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	listener, err := net.Listen("unix", DefaultSocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	// Make socket world-writable so any user can send notifications
	os.Chmod(DefaultSocketPath, 0666)

	return &NotifyServer{
		listener: listener,
		state:    state,
		stopChan: make(chan struct{}),
		clients:  make(chan struct{}, 4),
	}, nil
}

// Start begins listening for notifications in a goroutine
func (s *NotifyServer) Start() {
	go s.listen()
}

// Stop closes the notification server
func (s *NotifyServer) Stop() {
	close(s.stopChan)
	s.listener.Close()
	os.Remove(DefaultSocketPath)
}

func (s *NotifyServer) listen() {
	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		// Set accept timeout so we can check stopChan periodically
		s.listener.(*net.UnixListener).SetDeadline(time.Now().Add(1 * time.Second))

		conn, err := s.listener.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue // Timeout, check stopChan and try again
			}
			continue
		}

		select {
		case s.clients <- struct{}{}:
			go func() { defer func() { <-s.clients }(); s.handleConnection(conn) }()
		default:
			conn.Close()
		}
	}
}

func (s *NotifyServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewReader(io.LimitReader(conn, 4097))
	line, err := reader.ReadBytes('\n')
	if len(line) > 4096 || (err != nil && err != io.EOF) || len(line) == 0 {
		json.NewEncoder(conn).Encode(map[string]string{"error": "message must be valid JSON, at most 4096 bytes"})
		return
	}

	var msg SocketMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		json.NewEncoder(conn).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Handle different commands
	switch msg.Command {
	case "screenshot":
		s.handleScreenshot(conn)
	case "notify", "":
		// Default to notify for backwards compatibility
		s.handleNotify(conn, msg)
	default:
		json.NewEncoder(conn).Encode(map[string]string{"error": "unknown command: " + msg.Command})
	}
}

func (s *NotifyServer) handleNotify(conn net.Conn, msg SocketMessage) {
	if msg.Message == "" || utf8.RuneCountInString(msg.Message) > 512 || msg.Duration < 0 || msg.Duration > 86400 {
		conn.Write([]byte(`{"error":"message must contain 1-512 characters and duration must be 0-86400 seconds"}`))
		return
	}

	// Convert theme string to NotificationTheme
	theme := ThemeInfo
	switch msg.Theme {
	case "warning":
		theme = ThemeWarning
	case "danger":
		theme = ThemeDanger
	case "", "info":
	default:
		conn.Write([]byte(`{"error":"theme must be info, warning, or danger"}`))
		return
	}

	// Create notification
	notif := &Notification{
		Message: msg.Message,
		Theme:   theme,
	}

	// Set expiration if duration > 0
	if msg.Duration > 0 {
		notif.ExpiresAt = time.Now().Add(time.Duration(msg.Duration) * time.Second)
	}

	// Update state
	s.state.SetActiveNotification(notif)

	conn.Write([]byte(`{"status":"ok"}`))
}

func (s *NotifyServer) handleScreenshot(conn net.Conn) {
	frame := s.state.GetLastFrame()
	if frame == nil {
		conn.Write([]byte(`{"error":"no frame available"}`))
		return
	}

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, frame); err != nil {
		conn.Write([]byte(fmt.Sprintf(`{"error":"failed to encode PNG: %s"}`, err.Error())))
		return
	}

	// Send PNG data directly (binary)
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.Write(buf.Bytes())
}

// SendNotification sends a notification to a running wtr-display instance
func SendNotification(message, theme string, duration int) error {
	conn, err := net.DialTimeout("unix", DefaultSocketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to display (is it running?): %w", err)
	}
	defer conn.Close()

	msg := SocketMessage{
		Command:  "notify",
		Message:  message,
		Theme:    theme,
		Duration: duration,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Send with newline
	data = append(data, '\n')
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Read response
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	response, err := reader.ReadBytes('}')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(response, &resp); err != nil {
		return fmt.Errorf("invalid response: %s", string(response))
	}

	if resp.Error != "" {
		return fmt.Errorf("server error: %s", resp.Error)
	}

	return nil
}

// RequestScreenshot requests a screenshot from a running wtr-display instance
// and saves it to the specified file path. Returns error if failed.
func RequestScreenshot(outputPath string) error {
	conn, err := net.DialTimeout("unix", DefaultSocketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to display (is it running?): %w", err)
	}
	defer conn.Close()

	msg := SocketMessage{
		Command: "screenshot",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Send with newline
	data = append(data, '\n')
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Read response (PNG data or JSON error)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	response, err := io.ReadAll(conn)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check if it's an error response (starts with '{')
	if len(response) > 0 && response[0] == '{' {
		var resp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(response, &resp); err == nil && resp.Error != "" {
			return fmt.Errorf("server error: %s", resp.Error)
		}
	}

	// Check PNG magic bytes
	if len(response) < 8 || string(response[:8]) != "\x89PNG\r\n\x1a\n" {
		return fmt.Errorf("invalid response: not a PNG image")
	}

	// Write to file
	if err := os.WriteFile(outputPath, response, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
