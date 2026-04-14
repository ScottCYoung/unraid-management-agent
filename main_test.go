package main

import (
	"testing"

	"github.com/alecthomas/kong"
)

// TestCLIIntervalDefaults verifies that all collector interval defaults in the
// cliArgs struct match the values in scripts/start and unraid-agent-dev.page.
// This test exists to prevent silent drift between the three sources of truth.
func TestCLIIntervalDefaults(t *testing.T) {
	var c cliArgs
	p, err := kong.New(&c)
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	if _, err := p.Parse([]string{"boot"}); err != nil {
		t.Fatalf("kong.Parse: %v", err)
	}

	cases := []struct {
		name string
		got  int
		want int
	}{
		{"IntervalSystem", c.IntervalSystem, 30},
		{"IntervalArray", c.IntervalArray, 60},
		{"IntervalDisk", c.IntervalDisk, 300},
		{"IntervalDocker", c.IntervalDocker, 30},
		{"IntervalVM", c.IntervalVM, 60},
		{"IntervalUPS", c.IntervalUPS, 0},
		{"IntervalNUT", c.IntervalNUT, 0},
		{"IntervalGPU", c.IntervalGPU, 0},
		{"IntervalShares", c.IntervalShares, 300},
		{"IntervalNetwork", c.IntervalNetwork, 30},
		{"IntervalHardware", c.IntervalHardware, 3600},
		{"IntervalZFS", c.IntervalZFS, 0},
		{"IntervalNotification", c.IntervalNotification, 30},
		{"IntervalRegistration", c.IntervalRegistration, 3600},
		{"IntervalUnassigned", c.IntervalUnassigned, 300},
		{"IntervalFanControl", c.IntervalFanControl, 10},
		{"IntervalTuning", c.IntervalTuning, 300},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}
