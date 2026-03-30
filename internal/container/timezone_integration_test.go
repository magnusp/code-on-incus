package container

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// TestTimezoneApplyInContainer verifies that setting the timezone inside a
// container via /etc/localtime and /etc/timezone produces the expected result
// when running `date +%Z` and reading /etc/timezone.
func TestTimezoneApplyInContainer(t *testing.T) {
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}
	if !Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}
	exists, err := ImageExists("coi")
	if err != nil || !exists {
		t.Skip("coi image not found, skipping integration test (run 'coi build' first)")
	}

	containerName := "coi-test-timezone"
	mgr := NewManager(containerName)

	t.Cleanup(func() {
		_ = mgr.Stop(true)
		_ = mgr.Delete(true)
	})

	// Remove any leftover container from a previous run
	if exists, _ := mgr.Exists(); exists {
		_ = mgr.Stop(true)
		_ = mgr.Delete(true)
	}

	if err := mgr.Launch("coi", false); err != nil {
		t.Fatalf("Failed to launch container: %v", err)
	}

	tests := []struct {
		tz             string
		expectedInFile string // Expected content of /etc/timezone
	}{
		{"America/New_York", "America/New_York"},
		{"Europe/Warsaw", "Europe/Warsaw"},
		{"UTC", "UTC"},
	}

	for _, tt := range tests {
		t.Run(tt.tz, func(t *testing.T) {
			// Apply timezone (same command used in session/setup.go)
			tzCmd := fmt.Sprintf(
				"ln -sf /usr/share/zoneinfo/%s /etc/localtime && echo %s > /etc/timezone",
				tt.tz, tt.tz,
			)
			if _, err := mgr.ExecCommand(tzCmd, ExecCommandOptions{Capture: true}); err != nil {
				t.Fatalf("Failed to set timezone to %s: %v", tt.tz, err)
			}

			// Verify /etc/timezone contents
			output, err := mgr.ExecArgsCapture([]string{"cat", "/etc/timezone"}, ExecCommandOptions{})
			if err != nil {
				t.Fatalf("Failed to read /etc/timezone: %v", err)
			}
			got := strings.TrimSpace(output)
			if got != tt.expectedInFile {
				t.Errorf("/etc/timezone = %q, want %q", got, tt.expectedInFile)
			}

			// Verify /etc/localtime is a symlink pointing into zoneinfo
			// (use readlink without -f since some zones like UTC are symlinks themselves)
			linkOutput, err := mgr.ExecArgsCapture([]string{"readlink", "/etc/localtime"}, ExecCommandOptions{})
			if err != nil {
				t.Fatalf("Failed to readlink /etc/localtime: %v", err)
			}
			expectedLink := fmt.Sprintf("/usr/share/zoneinfo/%s", tt.tz)
			gotLink := strings.TrimSpace(linkOutput)
			if gotLink != expectedLink {
				t.Errorf("/etc/localtime -> %q, want %q", gotLink, expectedLink)
			}

			// Verify TZ env var works with date command
			envOutput, err := mgr.ExecCommand(
				fmt.Sprintf("TZ=%s date +%%Z", tt.tz),
				ExecCommandOptions{Capture: true},
			)
			if err != nil {
				t.Fatalf("Failed to run date with TZ=%s: %v", tt.tz, err)
			}
			tzAbbrev := strings.TrimSpace(envOutput)
			if tzAbbrev == "" {
				t.Errorf("date +%%Z with TZ=%s returned empty string", tt.tz)
			}
			t.Logf("TZ=%s -> date +%%Z = %s", tt.tz, tzAbbrev)
		})
	}
}

// TestTimezoneDetectAndApply verifies the full flow: detect host timezone,
// validate it, and apply it inside a container.
func TestTimezoneDetectAndApply(t *testing.T) {
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}
	if !Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}
	exists, err := ImageExists("coi")
	if err != nil || !exists {
		t.Skip("coi image not found, skipping integration test (run 'coi build' first)")
	}

	// Detect host timezone
	hostTZ, err := DetectHostTimezone()
	if err != nil || hostTZ == "" {
		t.Skip("Could not detect host timezone, skipping integration test")
	}
	t.Logf("Host timezone: %s", hostTZ)

	if !ValidateTimezone(hostTZ) {
		t.Fatalf("Detected timezone %q failed validation", hostTZ)
	}

	containerName := "coi-test-timezone-detect"
	mgr := NewManager(containerName)

	t.Cleanup(func() {
		_ = mgr.Stop(true)
		_ = mgr.Delete(true)
	})

	if exists, _ := mgr.Exists(); exists {
		_ = mgr.Stop(true)
		_ = mgr.Delete(true)
	}

	if err := mgr.Launch("coi", false); err != nil {
		t.Fatalf("Failed to launch container: %v", err)
	}

	// Apply detected host timezone
	tzCmd := fmt.Sprintf(
		"ln -sf /usr/share/zoneinfo/%s /etc/localtime && echo %s > /etc/timezone",
		hostTZ, hostTZ,
	)
	if _, err := mgr.ExecCommand(tzCmd, ExecCommandOptions{Capture: true}); err != nil {
		t.Fatalf("Failed to set timezone to %s: %v", hostTZ, err)
	}

	// Read back /etc/timezone — should match host
	output, err := mgr.ExecArgsCapture([]string{"cat", "/etc/timezone"}, ExecCommandOptions{})
	if err != nil {
		t.Fatalf("Failed to read /etc/timezone: %v", err)
	}
	got := strings.TrimSpace(output)
	if got != hostTZ {
		t.Errorf("Container /etc/timezone = %q, want host timezone %q", got, hostTZ)
	}

	// Run date in container with TZ env and on host, compare timezone abbreviation
	containerDate, err := mgr.ExecCommand(
		fmt.Sprintf("TZ=%s date +%%Z", hostTZ),
		ExecCommandOptions{Capture: true},
	)
	if err != nil {
		t.Fatalf("Failed to run date in container: %v", err)
	}

	hostDate, err := exec.Command("date", "+%Z").Output()
	if err != nil {
		t.Fatalf("Failed to run date on host: %v", err)
	}

	containerTZAbbrev := strings.TrimSpace(containerDate)
	hostTZAbbrev := strings.TrimSpace(string(hostDate))

	if containerTZAbbrev != hostTZAbbrev {
		t.Errorf("Container timezone abbreviation %q != host %q", containerTZAbbrev, hostTZAbbrev)
	}
	t.Logf("Both host and container report timezone abbreviation: %s", hostTZAbbrev)
}
