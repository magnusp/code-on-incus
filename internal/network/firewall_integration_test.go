package network

import (
	"os/exec"
	"testing"
)

// FirewallAvailable should return true only when firewall-cmd is installed AND
// firewalld is actually running.
func TestFirewallAvailable_Integration(t *testing.T) {
	_, lookPathErr := exec.LookPath("firewall-cmd")
	installed := lookPathErr == nil

	if !installed {
		t.Log("firewall-cmd not installed — expecting FirewallAvailable() == false")
		if FirewallAvailable() {
			t.Error("FirewallAvailable() returned true but firewall-cmd is not installed")
		}
		return
	}

	// firewall-cmd is installed; check if firewalld is running
	cmd := exec.Command("sudo", "-n", "firewall-cmd", "--state")
	running := cmd.Run() == nil

	result := FirewallAvailable()
	if result != running {
		t.Errorf("FirewallAvailable() = %v, but firewalld running state is %v", result, running)
	}

	t.Logf("FirewallAvailable: installed=%v running=%v result=%v", installed, running, result)
}

// FirewallInstalled should return true when firewall-cmd binary exists on PATH.
func TestFirewallInstalled_Integration(t *testing.T) {
	_, lookPathErr := exec.LookPath("firewall-cmd")
	expected := lookPathErr == nil

	result := FirewallInstalled()
	if result != expected {
		t.Errorf("FirewallInstalled() = %v, but exec.LookPath says %v", result, expected)
	}

	t.Logf("FirewallInstalled: %v", result)
}

// MasqueradeEnabled should return true only when firewalld is running and
// masquerade is configured.
func TestMasqueradeEnabled_Integration(t *testing.T) {
	if !FirewallAvailable() {
		t.Log("firewalld not available — expecting MasqueradeEnabled() == false")
		if MasqueradeEnabled() {
			t.Error("MasqueradeEnabled() returned true but firewalld is not available")
		}
		return
	}

	// Query masquerade directly to compare
	cmd := exec.Command("sudo", "-n", "firewall-cmd", "--query-masquerade")
	expected := cmd.Run() == nil

	result := MasqueradeEnabled()
	if result != expected {
		t.Errorf("MasqueradeEnabled() = %v, but direct query says %v", result, expected)
	}

	t.Logf("MasqueradeEnabled: %v", result)
}
