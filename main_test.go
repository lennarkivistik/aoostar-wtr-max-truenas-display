package main

import (
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCLIRejectsInvalidInputBeforeOpeningDevice(t *testing.T) {
	for _, args := range [][]string{
		{"--iface", "eth0", "--view", "network", "--views", "storage"},
		{"--iface", "eth0", "--frame-interval", "0s"},
		{"--iface", "eth0", "--wave-fps", "0"},
		{"--iface", "eth0", "--view-duration", "0s"},
		{"--iface", "eth0", "--samples", "-1"},
		{"--iface", "eth0", "--night-mode", "on", "--off-time", "25:00"},
	} {
		cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestCLIHelper", "--"}, args...)...)
		cmd.Env = append(os.Environ(), "WTR_CLI_TEST=1")
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "Error:") || strings.Contains(string(out), "Auto-detecting") {
			t.Fatalf("args %v: %s (%v)", args, out, err)
		}
	}
}

func TestCLIHelper(t *testing.T) {
	if os.Getenv("WTR_CLI_TEST") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"wtr-display"}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("wtr-display", flag.ExitOnError)
	main()
}
