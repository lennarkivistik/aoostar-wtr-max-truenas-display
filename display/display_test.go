package display

import (
	"testing"

	"go.bug.st/serial/enumerator"
)

func TestMatchesUSBVIDPID(t *testing.T) {
	tests := []struct {
		name string
		port *enumerator.PortDetails
		want bool
	}{
		{
			name: "matching USB port",
			port: &enumerator.PortDetails{IsUSB: true, VID: "0416", PID: "90A1"},
			want: true,
		},
		{
			name: "case insensitive match",
			port: &enumerator.PortDetails{IsUSB: true, VID: "0416", PID: "90a1"},
			want: true,
		},
		{
			name: "non USB port",
			port: &enumerator.PortDetails{IsUSB: false, VID: "0416", PID: "90A1"},
			want: false,
		},
		{
			name: "wrong VID",
			port: &enumerator.PortDetails{IsUSB: true, VID: "1234", PID: "90A1"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesUSBVIDPID(tt.port, "0416", "90A1"); got != tt.want {
				t.Fatalf("matchesUSBVIDPID() = %v, want %v", got, tt.want)
			}
		})
	}
}
