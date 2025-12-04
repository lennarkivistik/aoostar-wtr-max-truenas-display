package dashboard

import (
	"fmt"
	"time"
)

// Schedule represents night mode times when the display should be off.
type Schedule struct {
	OffHour   int // Hour to turn off (0-23)
	OffMinute int // Minute to turn off (0-59)
	OnHour    int // Hour to turn on (0-23)
	OnMinute  int // Minute to turn on (0-59)
}

// ParseSchedule parses off-time and on-time strings into a Schedule.
// Both must be provided for schedule to be active. Format: HH:MM
func ParseSchedule(offTime, onTime string) *Schedule {
	if offTime == "" || onTime == "" {
		return nil
	}

	offHour, offMin, err1 := parseTime(offTime)
	onHour, onMin, err2 := parseTime(onTime)

	if err1 != nil || err2 != nil {
		fmt.Printf("Warning: Invalid time format. Use HH:MM (e.g., 22:00)\n")
		return nil
	}

	return &Schedule{
		OffHour:   offHour,
		OffMinute: offMin,
		OnHour:    onHour,
		OnMinute:  onMin,
	}
}

// parseTime parses a HH:MM string into hour and minute.
func parseTime(s string) (int, int, error) {
	var hour, minute int
	_, err := fmt.Sscanf(s, "%d:%d", &hour, &minute)
	if err != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid time")
	}
	return hour, minute, nil
}

// IsNightMode checks if current time is within the off period.
// Handles schedules that cross midnight (e.g., 22:00 - 08:00).
func (s *Schedule) IsNightMode() bool {
	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()
	offMinutes := s.OffHour*60 + s.OffMinute
	onMinutes := s.OnHour*60 + s.OnMinute

	if offMinutes < onMinutes {
		// Same day: off 08:00, on 17:00 (off during work hours)
		return currentMinutes >= offMinutes && currentMinutes < onMinutes
	}
	// Crosses midnight: off 22:00, on 08:00 (off at night)
	return currentMinutes >= offMinutes || currentMinutes < onMinutes
}

