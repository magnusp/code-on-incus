package health

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/mensfeld/code-on-incus/internal/config"
	"github.com/mensfeld/code-on-incus/internal/container"
	"github.com/mensfeld/code-on-incus/internal/network"
	"github.com/mensfeld/code-on-incus/internal/nftmonitor"
)

// TestCheckIncus_VersionCheck verifies that CheckIncus returns version info
// and validates it against the minimum version requirement.
func TestCheckIncus_VersionCheck(t *testing.T) {
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	result := CheckIncus()

	if result.Name != "incus" {
		t.Errorf("Expected check name 'incus', got '%s'", result.Name)
	}

	// Should be OK or Failed (version too old)
	if result.Status != StatusOK && result.Status != StatusFailed {
		t.Errorf("Expected StatusOK or StatusFailed, got %s: %s", result.Status, result.Message)
	}

	// Details should contain a version
	if result.Details == nil || result.Details["version"] == nil {
		t.Error("Expected 'version' in details")
	}

	versionStr, ok := result.Details["version"].(string)
	if !ok || versionStr == "" {
		t.Errorf("Expected non-empty version string in details, got %v", result.Details["version"])
	}

	// Verify the version is parseable
	v, err := container.ParseIncusVersion(versionStr)
	if err != nil {
		t.Fatalf("Version from CheckIncus (%q) should be parseable: %v", versionStr, err)
	}

	if container.MeetsMinimumVersion(v) {
		if result.Status != StatusOK {
			t.Errorf("Version %s meets minimum but status is %s: %s", versionStr, result.Status, result.Message)
		}
		if !strings.Contains(result.Message, versionStr) {
			t.Errorf("Message should contain version %q, got %q", versionStr, result.Message)
		}
	} else {
		if result.Status != StatusFailed {
			t.Errorf("Version %s is below minimum but status is %s", versionStr, result.Status)
		}
		if !strings.Contains(result.Message, "zabbly") {
			t.Errorf("Failed message should mention zabbly, got %q", result.Message)
		}
	}

	t.Logf("CheckIncus: status=%s version=%s message=%s", result.Status, versionStr, result.Message)
}

// TestEvaluateIncusVersion_OldVersion verifies that old Incus versions
// produce a StatusFailed result with upgrade instructions.
func TestEvaluateIncusVersion_OldVersion(t *testing.T) {
	tests := []struct {
		name           string
		versionOutput  string
		expectStatus   CheckStatus
		expectContains string
	}{
		{
			"Ubuntu 6.0.x fails",
			"Client version: 6.0.3\nServer version: 6.0.3",
			StatusFailed,
			"zabbly",
		},
		{
			"6.0.0 fails",
			"Client version: 6.0.0\nServer version: 6.0.0",
			StatusFailed,
			"6.1",
		},
		{
			"5.x fails",
			"Server version: 5.21",
			StatusFailed,
			"zabbly",
		},
		{
			"6.1 passes",
			"Client version: 6.1\nServer version: 6.1",
			StatusOK,
			"6.1",
		},
		{
			"6.20 passes",
			"Client version: 6.20\nServer version: 6.20",
			StatusOK,
			"6.20",
		},
		{
			"7.0 passes",
			"Server version: 7.0",
			StatusOK,
			"7.0",
		},
		{
			"single line fallback",
			"6.0.1",
			StatusFailed,
			"zabbly",
		},
		{
			"unparseable degrades gracefully",
			"Server version: unknown-dev",
			StatusOK,
			"unknown-dev",
		},
		{
			"empty output degrades gracefully",
			"",
			StatusOK,
			"version unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateIncusVersion(tt.versionOutput)

			if result.Name != "incus" {
				t.Errorf("Expected check name 'incus', got %q", result.Name)
			}
			if result.Status != tt.expectStatus {
				t.Errorf("Expected status %s, got %s: %s", tt.expectStatus, result.Status, result.Message)
			}
			if !strings.Contains(result.Message, tt.expectContains) {
				t.Errorf("Expected message to contain %q, got %q", tt.expectContains, result.Message)
			}
		})
	}
}

// TestCheckNFTables_VersionCheck verifies that CheckNFTables returns version info
// and validates it against the minimum version requirement.
func TestCheckNFTables_VersionCheck(t *testing.T) {
	if _, err := exec.LookPath("nft"); err != nil {
		t.Skip("nft not found, skipping integration test")
	}

	result := CheckNFTables()

	if result.Name != "nftables" {
		t.Errorf("Expected check name 'nftables', got '%s'", result.Name)
	}

	// Details should contain a version
	if result.Details == nil || result.Details["version"] == nil {
		t.Skipf("No version in details (nft --version may have failed): %s", result.Message)
	}

	versionStr, ok := result.Details["version"].(string)
	if !ok || versionStr == "" {
		t.Errorf("Expected non-empty version string in details, got %v", result.Details["version"])
	}

	// Verify the version is parseable
	v, err := nftmonitor.ParseNFTVersion(versionStr)
	if err != nil {
		t.Fatalf("Version from CheckNFTables (%q) should be parseable: %v", versionStr, err)
	}

	if nftmonitor.MeetsMinimumNFTVersion(v) {
		// Version meets minimum — status should be OK (if sudo works) or Warning (sudo issue)
		if result.Status == StatusFailed {
			t.Errorf("Version %s meets minimum but status is Failed: %s", versionStr, result.Message)
		}
		if result.Status == StatusOK && !strings.Contains(result.Message, versionStr) {
			t.Errorf("OK message should contain version %q, got %q", versionStr, result.Message)
		}
	} else {
		if result.Status != StatusFailed {
			t.Errorf("Version %s is below minimum but status is %s", versionStr, result.Status)
		}
	}

	t.Logf("CheckNFTables: status=%s version=%s message=%s", result.Status, versionStr, result.Message)
}

// TestEvaluateNFTVersion_OldVersion verifies that old nftables versions
// produce a StatusFailed result with upgrade instructions.
func TestEvaluateNFTVersion_OldVersion(t *testing.T) {
	tests := []struct {
		name           string
		versionOutput  string
		expectFailed   bool
		expectContains string
	}{
		{
			"0.8.3 fails",
			"nftables v0.8.3 (Topsy)",
			true,
			"0.9.0",
		},
		{
			"0.7.0 fails",
			"nftables v0.7.0 (Scruffy)",
			true,
			"nftables",
		},
		{
			"0.9.0 passes",
			"nftables v0.9.0 (Fearless Fosdick)",
			false,
			"",
		},
		{
			"1.0.9 passes",
			"nftables v1.0.9 (Old Doc Yak #3)",
			false,
			"",
		},
		{
			"unparseable returns nil",
			"some garbage output",
			false,
			"",
		},
		{
			"empty returns nil",
			"",
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateNFTVersion("/usr/sbin/nft", tt.versionOutput)

			if tt.expectFailed {
				if result == nil {
					t.Fatal("Expected a failed HealthCheck, got nil")
				}
				if result.Status != StatusFailed {
					t.Errorf("Expected StatusFailed, got %s: %s", result.Status, result.Message)
				}
				if !strings.Contains(result.Message, tt.expectContains) {
					t.Errorf("Expected message to contain %q, got %q", tt.expectContains, result.Message)
				}
			} else {
				if result != nil {
					t.Errorf("Expected nil (version OK or unparseable), got status=%s: %s", result.Status, result.Message)
				}
			}
		})
	}
}

// TestCheckContainerConnectivity_NoImage verifies that the check is skipped
// when the specified image doesn't exist.
func TestCheckContainerConnectivity_NoImage(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// Use a non-existent image name
	result := CheckContainerConnectivity("non-existent-image-12345")

	if result.Name != "container_connectivity" {
		t.Errorf("Expected check name 'container_connectivity', got '%s'", result.Name)
	}

	if result.Status != StatusWarning {
		t.Errorf("Expected StatusWarning when image doesn't exist, got %s", result.Status)
	}

	if !strings.Contains(result.Message, "Skipped") || !strings.Contains(result.Message, "image not available") {
		t.Errorf("Expected message about skipped/image not available, got '%s'", result.Message)
	}

	t.Logf("Correctly skipped check for non-existent image: %s", result.Message)
}

// TestCheckContainerConnectivity_WithImage verifies the full connectivity check
// when a valid image exists. This test actually launches a container.
func TestCheckContainerConnectivity_WithImage(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// Check if the default 'coi' image exists
	exists, err := container.ImageExists("coi")
	if err != nil || !exists {
		t.Skip("coi image not found, skipping integration test (run 'coi build' first)")
	}

	// Run the actual connectivity check
	result := CheckContainerConnectivity("coi")

	if result.Name != "container_connectivity" {
		t.Errorf("Expected check name 'container_connectivity', got '%s'", result.Name)
	}

	// The check should complete (not hang) and return a definitive status
	switch result.Status {
	case StatusOK:
		t.Logf("Container connectivity check passed: %s", result.Message)
		if result.Details != nil {
			t.Logf("Details: dns_test=%v, http_test=%v", result.Details["dns_test"], result.Details["http_test"])
		}
	case StatusWarning:
		t.Logf("Container connectivity check warning: %s", result.Message)
	case StatusFailed:
		t.Logf("Container connectivity check failed (expected if network is misconfigured): %s", result.Message)
	default:
		t.Errorf("Unexpected status: %s", result.Status)
	}

	// Verify no leftover containers
	containers, err := container.ListContainers("^coi-health-check-")
	if err != nil {
		t.Errorf("Failed to list containers: %v", err)
	}
	if len(containers) > 0 {
		t.Errorf("Found leftover health check containers: %v", containers)
	}
}

// TestCheckContainerConnectivity_EmptyImageName verifies that empty image name
// defaults to "coi".
func TestCheckContainerConnectivity_EmptyImageName(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// Check if the default 'coi' image exists
	exists, err := container.ImageExists("coi")
	if err != nil {
		t.Skip("could not check for coi image, skipping integration test")
	}

	// Run with empty image name
	result := CheckContainerConnectivity("")

	if result.Name != "container_connectivity" {
		t.Errorf("Expected check name 'container_connectivity', got '%s'", result.Name)
	}

	if !exists {
		// Should be skipped if image doesn't exist
		if result.Status != StatusWarning {
			t.Errorf("Expected StatusWarning when default image doesn't exist, got %s", result.Status)
		}
		t.Logf("Correctly handled missing default image: %s", result.Message)
	} else {
		// Should run the check if image exists
		if result.Status == StatusWarning && strings.Contains(result.Message, "Skipped") {
			t.Errorf("Should not skip when default coi image exists, got: %s", result.Message)
		}
		t.Logf("Check ran with default image: status=%s, message=%s", result.Status, result.Message)
	}
}

// TestCheckContainerConnectivity_Cleanup verifies that containers are cleaned up
// even when the check fails or encounters errors.
func TestCheckContainerConnectivity_Cleanup(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// Check if the default 'coi' image exists
	exists, err := container.ImageExists("coi")
	if err != nil || !exists {
		t.Skip("coi image not found, skipping integration test")
	}

	// Count containers before
	containersBefore, _ := container.ListContainers("^coi-health-check-")

	// Run multiple checks to ensure cleanup works
	for i := 0; i < 3; i++ {
		_ = CheckContainerConnectivity("coi")
	}

	// Count containers after
	containersAfter, err := container.ListContainers("^coi-health-check-")
	if err != nil {
		t.Errorf("Failed to list containers: %v", err)
	}

	if len(containersAfter) > len(containersBefore) {
		t.Errorf("Found %d new leftover containers after running checks: %v",
			len(containersAfter)-len(containersBefore), containersAfter)
	} else {
		t.Logf("Cleanup verified: no leftover containers after %d checks", 3)
	}
}

// TestCheckNetworkRestriction_NoFirewall verifies that the check is skipped
// when firewalld is not available.
func TestCheckNetworkRestriction_NoFirewall(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// This test only makes sense if firewall is NOT available
	if network.FirewallAvailable() {
		t.Skip("firewalld is available, cannot test no-firewall scenario")
	}

	result := CheckNetworkRestriction("coi")

	if result.Name != "network_restriction" {
		t.Errorf("Expected check name 'network_restriction', got '%s'", result.Name)
	}

	if result.Status != StatusWarning {
		t.Errorf("Expected StatusWarning when firewall not available, got %s", result.Status)
	}

	if !strings.Contains(result.Message, "firewalld not available") {
		t.Errorf("Expected message about firewalld not available, got '%s'", result.Message)
	}

	t.Logf("Correctly skipped check when firewall unavailable: %s", result.Message)
}

// TestCheckNetworkRestriction_NoImage verifies that the check is skipped
// when the specified image doesn't exist.
func TestCheckNetworkRestriction_NoImage(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// Skip if firewall is not available
	if !network.FirewallAvailable() {
		t.Skip("firewalld not available, skipping integration test")
	}

	// Use a non-existent image name
	result := CheckNetworkRestriction("non-existent-image-12345")

	if result.Name != "network_restriction" {
		t.Errorf("Expected check name 'network_restriction', got '%s'", result.Name)
	}

	if result.Status != StatusWarning {
		t.Errorf("Expected StatusWarning when image doesn't exist, got %s", result.Status)
	}

	if !strings.Contains(result.Message, "image not available") {
		t.Errorf("Expected message about image not available, got '%s'", result.Message)
	}

	t.Logf("Correctly skipped check for non-existent image: %s", result.Message)
}

// TestCheckNetworkRestriction_WithImage verifies the full network restriction check
// when firewall and image are available.
func TestCheckNetworkRestriction_WithImage(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// Skip if firewall is not available
	if !network.FirewallAvailable() {
		t.Skip("firewalld not available, skipping integration test")
	}

	// Check if the default 'coi' image exists
	exists, err := container.ImageExists("coi")
	if err != nil || !exists {
		t.Skip("coi image not found, skipping integration test (run 'coi build' first)")
	}

	// Run the network restriction check
	result := CheckNetworkRestriction("coi")

	if result.Name != "network_restriction" {
		t.Errorf("Expected check name 'network_restriction', got '%s'", result.Name)
	}

	// The check should complete and return a definitive status
	switch result.Status {
	case StatusOK:
		t.Logf("Network restriction check passed: %s", result.Message)
		if result.Details != nil {
			t.Logf("Details: external_access=%v, private_blocked=%v",
				result.Details["external_access"], result.Details["private_blocked"])
		}
	case StatusWarning:
		t.Logf("Network restriction check warning: %s", result.Message)
	case StatusFailed:
		t.Logf("Network restriction check failed: %s", result.Message)
		if result.Details != nil {
			t.Logf("Details: %v", result.Details)
		}
	default:
		t.Errorf("Unexpected status: %s", result.Status)
	}

	// Verify no leftover containers
	containers, err := container.ListContainers("^coi-restriction-check-")
	if err != nil {
		t.Errorf("Failed to list containers: %v", err)
	}
	if len(containers) > 0 {
		t.Errorf("Found leftover restriction check containers: %v", containers)
	}
}

// TestCheckNetworkRestriction_Cleanup verifies that containers and firewall rules
// are cleaned up after the check.
func TestCheckNetworkRestriction_Cleanup(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// Skip if firewall is not available
	if !network.FirewallAvailable() {
		t.Skip("firewalld not available, skipping integration test")
	}

	// Check if the default 'coi' image exists
	exists, err := container.ImageExists("coi")
	if err != nil || !exists {
		t.Skip("coi image not found, skipping integration test")
	}

	// Count containers before
	containersBefore, _ := container.ListContainers("^coi-restriction-check-")

	// Run the check
	_ = CheckNetworkRestriction("coi")

	// Count containers after
	containersAfter, err := container.ListContainers("^coi-restriction-check-")
	if err != nil {
		t.Errorf("Failed to list containers: %v", err)
	}

	if len(containersAfter) > len(containersBefore) {
		t.Errorf("Found %d new leftover containers after check: %v",
			len(containersAfter)-len(containersBefore), containersAfter)
	} else {
		t.Logf("Cleanup verified: no leftover containers after restriction check")
	}
}

// TestCheckAuditLogDirectory verifies audit log directory check
func TestCheckAuditLogDirectory(t *testing.T) {
	result := CheckAuditLogDirectory()

	if result.Name != "audit_log_directory" {
		t.Errorf("Expected check name 'audit_log_directory', got '%s'", result.Name)
	}

	if result.Status != StatusOK {
		t.Errorf("Expected StatusOK for audit log directory, got %s: %s",
			result.Status, result.Message)
	}

	if result.Details["path"] == nil {
		t.Error("Expected 'path' in details")
	}

	t.Logf("Audit log directory check passed: %s", result.Message)
}

// TestCheckCgroupAvailability verifies cgroup availability check
func TestCheckCgroupAvailability(t *testing.T) {
	result := CheckCgroupAvailability()

	if result.Name != "cgroup_availability" {
		t.Errorf("Expected check name 'cgroup_availability', got '%s'", result.Name)
	}

	// Should be OK or Warning (v1 vs v2)
	if result.Status != StatusOK && result.Status != StatusWarning {
		t.Errorf("Expected StatusOK or StatusWarning for cgroups, got %s: %s",
			result.Status, result.Message)
	}

	if result.Details["path"] == nil {
		t.Error("Expected 'path' in details")
	}

	t.Logf("Cgroup availability check: %s (status: %s)", result.Message, result.Status)
}

// TestCheckMonitoringConfiguration verifies monitoring configuration check
func TestCheckMonitoringConfiguration(t *testing.T) {
	// Use default config
	cfg := config.GetDefaultConfig()

	result := CheckMonitoringConfiguration(cfg)

	if result.Name != "monitoring_configuration" {
		t.Errorf("Expected check name 'monitoring_configuration', got '%s'", result.Name)
	}

	// Should be OK or Warning depending on if monitoring is enabled
	if result.Status != StatusOK && result.Status != StatusWarning {
		t.Errorf("Expected StatusOK or StatusWarning for monitoring config, got %s: %s",
			result.Status, result.Message)
	}

	if result.Details["enabled"] == nil {
		t.Error("Expected 'enabled' in details")
	}

	t.Logf("Monitoring configuration check: %s (enabled: %v)",
		result.Message, result.Details["enabled"])
}

// TestCheckProcessMonitoringCapability verifies process monitoring capability check
func TestCheckProcessMonitoringCapability(t *testing.T) {
	// Skip if incus is not available
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}

	// Skip if incus daemon is not running
	if !container.Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	// Get default image
	cfg := config.GetDefaultConfig()

	result := CheckProcessMonitoringCapability(cfg.Defaults.Image)

	if result.Name != "process_monitoring" {
		t.Errorf("Expected check name 'process_monitoring', got '%s'", result.Name)
	}

	// Could be OK, Warning, or Failed depending on environment
	if result.Status != StatusOK && result.Status != StatusWarning && result.Status != StatusFailed {
		t.Errorf("Unexpected status: %s", result.Status)
	}

	t.Logf("Process monitoring capability check: %s (status: %s)",
		result.Message, result.Status)
}
