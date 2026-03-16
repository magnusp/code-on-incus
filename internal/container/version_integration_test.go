package container

import (
	"os/exec"
	"testing"
)

func TestIncusVersionIntegration(t *testing.T) {
	if _, err := exec.LookPath("incus"); err != nil {
		t.Skip("incus not found, skipping integration test")
	}
	if !Available() {
		t.Skip("incus daemon not running, skipping integration test")
	}

	output, err := IncusOutput("version")
	if err != nil {
		t.Fatalf("incus version failed: %v", err)
	}

	versionStr, err := ExtractServerVersion(output)
	if err != nil {
		t.Fatalf("ExtractServerVersion failed on real output %q: %v", output, err)
	}

	v, err := ParseIncusVersion(versionStr)
	if err != nil {
		t.Fatalf("ParseIncusVersion failed on %q: %v", versionStr, err)
	}

	t.Logf("Parsed Incus version: major=%d minor=%d raw=%q", v.Major, v.Minor, v.Raw)

	if v.Major < 5 || v.Major > 99 {
		t.Errorf("major version %d outside reasonable range [5, 99]", v.Major)
	}
}
