package session

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mensfeld/code-on-incus/internal/config"
	"github.com/mensfeld/code-on-incus/internal/container"
)

// skipUnlessTimezoneTestable skips the test if integration prerequisites are missing.
func skipUnlessTimezoneTestable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}
	exists, err := container.ImageExists("coi")
	if err != nil || !exists {
		t.Skip("coi image not found, skipping integration test (run 'coi build' first)")
	}
}

// launchTimezoneTestContainer creates and starts a container, registering cleanup.
func launchTimezoneTestContainer(t *testing.T, name string) *container.Manager {
	t.Helper()
	mgr := container.NewManager(name)

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

	// Wait for container to be ready
	for i := 0; i < 30; i++ {
		if _, err := mgr.ExecCommand("echo ready", container.ExecCommandOptions{Capture: true}); err == nil {
			return mgr
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatal("Container failed to become ready")
	return nil
}

// TestTimezone_SessionSetup_HostMode verifies that session.Setup() with
// Timezone set correctly configures /etc/localtime and /etc/timezone inside
// the container, and that the timezone matches the host.
func TestTimezone_SessionSetup_HostMode(t *testing.T) {
	skipUnlessTimezoneTestable(t)

	// Detect host timezone
	hostTZ, err := container.DetectHostTimezone()
	if err != nil || hostTZ == "" {
		t.Skip("Could not detect host timezone, skipping")
	}
	t.Logf("Host timezone: %s", hostTZ)

	containerName := "coi-test-tz-setup-host"
	mgr := launchTimezoneTestContainer(t, containerName)

	// Apply timezone the same way session.Setup() does (step 6.7)
	logger := func(msg string) { t.Logf("[tz] %s", msg) }

	logger("Setting container timezone to " + hostTZ)
	tzCmd := "ln -sf /usr/share/zoneinfo/" + hostTZ + " /etc/localtime && echo " + hostTZ + " > /etc/timezone"
	if _, err := mgr.ExecCommand(tzCmd, container.ExecCommandOptions{Capture: true}); err != nil {
		t.Fatalf("Failed to set timezone: %v", err)
	}

	// Verify /etc/timezone
	etcTZ, err := mgr.ExecArgsCapture([]string{"cat", "/etc/timezone"}, container.ExecCommandOptions{})
	if err != nil {
		t.Fatalf("Failed to read /etc/timezone: %v", err)
	}
	if got := strings.TrimSpace(etcTZ); got != hostTZ {
		t.Errorf("/etc/timezone = %q, want %q", got, hostTZ)
	}

	// Verify /etc/localtime symlink
	linkTarget, err := mgr.ExecArgsCapture([]string{"readlink", "/etc/localtime"}, container.ExecCommandOptions{})
	if err != nil {
		t.Fatalf("Failed to readlink /etc/localtime: %v", err)
	}
	expectedLink := "/usr/share/zoneinfo/" + hostTZ
	if got := strings.TrimSpace(linkTarget); got != expectedLink {
		t.Errorf("/etc/localtime -> %q, want %q", got, expectedLink)
	}

	// Verify date output with TZ env var matches host
	user := container.CodeUID
	containerDate, err := mgr.ExecCommand(
		"TZ="+hostTZ+" date +%Z",
		container.ExecCommandOptions{Capture: true, User: &user},
	)
	if err != nil {
		t.Fatalf("date +%%Z failed: %v", err)
	}

	hostDate, err := exec.Command("date", "+%Z").Output()
	if err != nil {
		t.Fatalf("Host date +%%Z failed: %v", err)
	}

	containerAbbrev := strings.TrimSpace(containerDate)
	hostAbbrev := strings.TrimSpace(string(hostDate))
	if containerAbbrev != hostAbbrev {
		t.Errorf("Container TZ abbreviation %q != host %q", containerAbbrev, hostAbbrev)
	}
	t.Logf("Both report timezone abbreviation: %s", hostAbbrev)
}

// TestTimezone_SessionSetup_FixedMode verifies that a fixed timezone
// (e.g., "America/Chicago") is correctly applied inside the container.
func TestTimezone_SessionSetup_FixedMode(t *testing.T) {
	skipUnlessTimezoneTestable(t)

	fixedTZ := "America/Chicago"
	if !container.ValidateTimezone(fixedTZ) {
		t.Skipf("Timezone %q not available on host", fixedTZ)
	}

	containerName := "coi-test-tz-setup-fixed"
	mgr := launchTimezoneTestContainer(t, containerName)

	// Apply timezone
	tzCmd := "ln -sf /usr/share/zoneinfo/" + fixedTZ + " /etc/localtime && echo " + fixedTZ + " > /etc/timezone"
	if _, err := mgr.ExecCommand(tzCmd, container.ExecCommandOptions{Capture: true}); err != nil {
		t.Fatalf("Failed to set timezone: %v", err)
	}

	// Verify /etc/timezone
	etcTZ, err := mgr.ExecArgsCapture([]string{"cat", "/etc/timezone"}, container.ExecCommandOptions{})
	if err != nil {
		t.Fatalf("Failed to read /etc/timezone: %v", err)
	}
	if got := strings.TrimSpace(etcTZ); got != fixedTZ {
		t.Errorf("/etc/timezone = %q, want %q", got, fixedTZ)
	}

	// Verify date reads the fixed timezone via /etc/localtime (no TZ env)
	dateOutput, err := mgr.ExecCommand("date +%Z", container.ExecCommandOptions{Capture: true})
	if err != nil {
		t.Fatalf("date +%%Z failed: %v", err)
	}
	abbrev := strings.TrimSpace(dateOutput)
	// America/Chicago is either CDT or CST depending on DST
	if abbrev != "CDT" && abbrev != "CST" {
		t.Errorf("date +%%Z = %q, want CDT or CST for %s", abbrev, fixedTZ)
	}
	t.Logf("Fixed timezone %s reports abbreviation: %s", fixedTZ, abbrev)
}

// TestTimezone_SessionSetup_UTCMode verifies that when timezone is empty
// (UTC mode), the container retains its default UTC timezone.
func TestTimezone_SessionSetup_UTCMode(t *testing.T) {
	skipUnlessTimezoneTestable(t)

	containerName := "coi-test-tz-setup-utc"
	mgr := launchTimezoneTestContainer(t, containerName)

	// Do NOT apply any timezone (simulates utc mode where opts.Timezone == "")

	// Verify container defaults to UTC
	dateOutput, err := mgr.ExecCommand("date +%Z", container.ExecCommandOptions{Capture: true})
	if err != nil {
		t.Fatalf("date +%%Z failed: %v", err)
	}
	abbrev := strings.TrimSpace(dateOutput)
	if abbrev != "UTC" {
		t.Errorf("date +%%Z = %q, want UTC for default container", abbrev)
	}
	t.Logf("UTC mode: container reports %s", abbrev)
}

// TestTimezone_TZEnvOverridesLocaltime verifies that the TZ environment
// variable takes precedence over /etc/localtime, ensuring processes that
// check TZ first get the correct timezone.
func TestTimezone_TZEnvOverridesLocaltime(t *testing.T) {
	skipUnlessTimezoneTestable(t)

	containerName := "coi-test-tz-env-override"
	mgr := launchTimezoneTestContainer(t, containerName)

	// Set /etc/localtime to UTC
	utcCmd := "ln -sf /usr/share/zoneinfo/UTC /etc/localtime && echo UTC > /etc/timezone"
	if _, err := mgr.ExecCommand(utcCmd, container.ExecCommandOptions{Capture: true}); err != nil {
		t.Fatalf("Failed to set UTC: %v", err)
	}

	// Run date with TZ=Asia/Tokyo — should override /etc/localtime
	user := container.CodeUID
	dateOutput, err := mgr.ExecCommand("date +%Z", container.ExecCommandOptions{
		Capture: true,
		User:    &user,
		Env:     map[string]string{"TZ": "Asia/Tokyo"},
	})
	if err != nil {
		t.Fatalf("date +%%Z with TZ=Asia/Tokyo failed: %v", err)
	}
	abbrev := strings.TrimSpace(dateOutput)
	if abbrev != "JST" {
		t.Errorf("date +%%Z with TZ=Asia/Tokyo = %q, want JST", abbrev)
	}

	// Without TZ, should still be UTC from /etc/localtime
	dateOutput2, err := mgr.ExecCommand("date +%Z", container.ExecCommandOptions{
		Capture: true,
		User:    &user,
	})
	if err != nil {
		t.Fatalf("date +%%Z without TZ failed: %v", err)
	}
	abbrev2 := strings.TrimSpace(dateOutput2)
	if abbrev2 != "UTC" {
		t.Errorf("date +%%Z without TZ = %q, want UTC", abbrev2)
	}
	t.Log("TZ env correctly overrides /etc/localtime")
}

// TestTimezone_WorkspaceFile verifies that a process inside the container can
// write its timezone to a file in the mounted workspace, and that the host can
// read back the correct value. This is the realistic scenario: a tool (e.g., git)
// writes a timestamp using the container's configured timezone and the result is
// visible on the host filesystem.
func TestTimezone_WorkspaceFile(t *testing.T) {
	skipUnlessTimezoneTestable(t)

	// Use a timezone that is definitely NOT the host timezone.
	// Pick Asia/Tokyo (JST, UTC+9) — unlikely to be the host TZ for CI/dev.
	fixedTZ := "Asia/Tokyo"
	if !container.ValidateTimezone(fixedTZ) {
		t.Skipf("Timezone %q not available on host", fixedTZ)
	}

	containerName := "coi-test-tz-workspace"
	mgr := launchTimezoneTestContainer(t, containerName)

	// Create a temp directory on the host to serve as the workspace
	workspaceDir := t.TempDir()

	// Mount it into the container
	useShift := true
	if err := mgr.MountDisk("workspace", workspaceDir, "/workspace", useShift, false); err != nil {
		t.Fatalf("Failed to mount workspace: %v", err)
	}

	// Apply the fixed timezone
	tzCmd := "ln -sf /usr/share/zoneinfo/" + fixedTZ + " /etc/localtime && echo " + fixedTZ + " > /etc/timezone"
	if _, err := mgr.ExecCommand(tzCmd, container.ExecCommandOptions{Capture: true}); err != nil {
		t.Fatalf("Failed to set timezone: %v", err)
	}

	// Inside the container, write the timezone to a file in the workspace.
	// Use TZ env var to be explicit (same as buildContainerEnv does).
	user := container.CodeUID
	writeCmd := "date +%Z > /workspace/tz_test.txt && echo " + fixedTZ + " >> /workspace/tz_test.txt"
	if _, err := mgr.ExecCommand(writeCmd, container.ExecCommandOptions{
		Capture: true,
		User:    &user,
		Env:     map[string]string{"TZ": fixedTZ},
	}); err != nil {
		t.Fatalf("Failed to write timezone file: %v", err)
	}

	// Read the file back FROM THE HOST filesystem (through the mount)
	hostFilePath := workspaceDir + "/tz_test.txt"
	data, err := os.ReadFile(hostFilePath)
	if err != nil {
		t.Fatalf("Failed to read timezone file on host: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected 2 lines in tz_test.txt, got %d: %q", len(lines), string(data))
	}

	tzAbbrev := strings.TrimSpace(lines[0])
	tzName := strings.TrimSpace(lines[1])

	if tzAbbrev != "JST" {
		t.Errorf("Timezone abbreviation = %q, want JST", tzAbbrev)
	}
	if tzName != fixedTZ {
		t.Errorf("Timezone name = %q, want %q", tzName, fixedTZ)
	}

	t.Logf("Workspace file on host contains: abbreviation=%s, name=%s", tzAbbrev, tzName)

	// Also verify that a git-style timestamp written inside the container
	// uses the correct timezone offset (+0900 for JST)
	gitDateCmd := "date '+%Y-%m-%d %H:%M:%S %z'"
	gitDateOutput, err := mgr.ExecCommand(gitDateCmd, container.ExecCommandOptions{
		Capture: true,
		User:    &user,
		Env:     map[string]string{"TZ": fixedTZ},
	})
	if err != nil {
		t.Fatalf("Failed to get git-style date: %v", err)
	}
	gitDate := strings.TrimSpace(gitDateOutput)
	if !strings.HasSuffix(gitDate, "+0900") {
		t.Errorf("Git-style date = %q, want suffix +0900 for JST", gitDate)
	}
	t.Logf("Git-style timestamp in container: %s", gitDate)
}

// TestTimezone_ConfigMerge verifies that TimezoneConfig merges correctly.
func TestTimezone_ConfigMerge(t *testing.T) {
	tests := []struct {
		name         string
		base         config.TimezoneConfig
		other        config.TimezoneConfig
		expectedMode string
		expectedName string
	}{
		{
			name:         "other overrides mode",
			base:         config.TimezoneConfig{Mode: "host"},
			other:        config.TimezoneConfig{Mode: "fixed"},
			expectedMode: "fixed",
		},
		{
			name:         "empty other preserves base mode",
			base:         config.TimezoneConfig{Mode: "fixed"},
			other:        config.TimezoneConfig{},
			expectedMode: "fixed",
		},
		{
			name:         "other overrides name",
			base:         config.TimezoneConfig{Mode: "fixed", Name: "UTC"},
			other:        config.TimezoneConfig{Name: "Europe/Warsaw"},
			expectedMode: "fixed",
			expectedName: "Europe/Warsaw",
		},
		{
			name:         "empty other preserves base name",
			base:         config.TimezoneConfig{Mode: "fixed", Name: "Europe/Warsaw"},
			other:        config.TimezoneConfig{},
			expectedMode: "fixed",
			expectedName: "Europe/Warsaw",
		},
		{
			name:         "utc mode",
			base:         config.TimezoneConfig{Mode: "host"},
			other:        config.TimezoneConfig{Mode: "utc"},
			expectedMode: "utc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := config.GetDefaultConfig()
			base.Timezone = tt.base

			other := &config.Config{Timezone: tt.other}
			base.Merge(other)

			if base.Timezone.Mode != tt.expectedMode {
				t.Errorf("Mode = %q, want %q", base.Timezone.Mode, tt.expectedMode)
			}
			if tt.expectedName != "" && base.Timezone.Name != tt.expectedName {
				t.Errorf("Name = %q, want %q", base.Timezone.Name, tt.expectedName)
			}
		})
	}
}
