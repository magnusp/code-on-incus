package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// skipUnlessIntegration skips the test unless incus is available and the coi image exists.
func skipUnlessIntegration(t *testing.T) {
	t.Helper()
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
}

// hostUID returns the UID of the current process.
func hostUID() int {
	return os.Getuid()
}

// initTestContainer creates a stopped container from the coi image, enables Docker
// support (required security flags), and registers cleanup. The caller can add
// config/devices before starting the container with mgr.Start().
func initTestContainer(t *testing.T, containerName string) *Manager {
	t.Helper()
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

	// Init (don't start yet — caller will add config/devices first)
	if err := IncusExec("init", "coi", containerName); err != nil {
		t.Fatalf("Failed to init container: %v", err)
	}

	// Enable Docker/nesting support (must be set before first boot)
	if err := EnableDockerSupport(containerName); err != nil {
		t.Fatalf("Failed to enable docker support: %v", err)
	}

	return mgr
}

// testWorkspaceWriteAccess runs a series of read/write operations as the code user
// (UID 1000) inside the container's /workspace and returns any failure messages.
func testWorkspaceWriteAccess(t *testing.T, mgr *Manager) []string {
	t.Helper()
	codeUID := CodeUID
	var failures []string

	// 1. Read the host-created file
	output, err := mgr.ExecArgsCapture(
		[]string{"cat", "/workspace/host-file.txt"},
		ExecCommandOptions{User: &codeUID, Group: &codeUID},
	)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read host file: %v", err))
	} else if got := strings.TrimSpace(output); got != "written-by-host" {
		failures = append(failures, fmt.Sprintf("host file content = %q, want %q", got, "written-by-host"))
	}

	// 2. Create a new file
	_, err = mgr.ExecArgsCapture(
		[]string{"sh", "-c", "echo created-by-code > /workspace/code-file.txt"},
		ExecCommandOptions{User: &codeUID, Group: &codeUID},
	)
	if err != nil {
		failures = append(failures, fmt.Sprintf("create new file: %v", err))
	}

	// 3. Overwrite the host-created file
	_, err = mgr.ExecArgsCapture(
		[]string{"sh", "-c", "echo overwritten-by-code > /workspace/host-file.txt"},
		ExecCommandOptions{User: &codeUID, Group: &codeUID},
	)
	if err != nil {
		failures = append(failures, fmt.Sprintf("overwrite host file: %v", err))
	}

	return failures
}

// TestWorkspaceWriteAccess_ShiftTrue verifies that workspace bind mounts with
// shift=true allow the container's code user (UID 1000) to read and write files.
//
// This test demonstrates a known bug: shift=true only works when the host user's
// UID matches the container's code user UID (1000). On CI runners where the host
// UID is typically 1001, shift=true fails and files are inaccessible to the code user.
//
// On machines where the host UID is 1000, this test passes — the bug only manifests
// when host UID ≠ 1000.
func TestWorkspaceWriteAccess_ShiftTrue(t *testing.T) {
	skipUnlessIntegration(t)

	uid := hostUID()
	t.Logf("Host UID: %d (shift=true %s when host UID ≠ %d)",
		uid, "fails", CodeUID)

	containerName := "coi-test-uid-shift"
	mgr := initTestContainer(t, containerName)

	// Create a temp dir with a host-owned file to mount as workspace
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/host-file.txt", []byte("written-by-host"), 0o644); err != nil {
		t.Fatalf("Failed to create host file: %v", err)
	}

	// Mount workspace with shift=true
	if err := mgr.MountDisk("workspace", tmpDir, "/workspace", true, false); err != nil {
		t.Fatalf("Failed to mount workspace: %v", err)
	}

	// Start the container
	if err := mgr.Start(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Test read/write access as code user
	failures := testWorkspaceWriteAccess(t, mgr)

	if uid == CodeUID {
		// On a host with UID 1000, shift=true works — all operations should succeed
		for _, f := range failures {
			t.Errorf("Unexpected failure (host UID matches container UID): %s", f)
		}
	} else {
		// On a host with UID ≠ 1000, shift=true fails — this IS the bug
		if len(failures) > 0 {
			t.Logf("Expected failures with shift=true (host UID %d ≠ container UID %d):", uid, CodeUID)
			for _, f := range failures {
				t.Logf("  - %s", f)
			}
		} else {
			t.Logf("Surprisingly, shift=true worked even with host UID %d — "+
				"the kernel may support idmap mounts", uid)
		}
	}
}

// TestWorkspaceWriteAccess_RawIdmap verifies that workspace bind mounts with
// raw.idmap correctly map the host user's UID to the container's code user UID,
// allowing full read/write access regardless of host UID.
//
// This validates the fix for the shift=true bug: raw.idmap explicitly maps
// "both <hostUID> 1000", so the container's code user always sees files as its own.
func TestWorkspaceWriteAccess_RawIdmap(t *testing.T) {
	skipUnlessIntegration(t)

	uid := hostUID()
	t.Logf("Host UID: %d → mapping to container UID %d via raw.idmap", uid, CodeUID)

	containerName := "coi-test-uid-idmap"
	mgr := initTestContainer(t, containerName)

	// Set raw.idmap: map host UID to container code user UID
	idmapValue := fmt.Sprintf("both %d %d", uid, CodeUID)
	if err := IncusExec("config", "set", containerName, "raw.idmap", idmapValue); err != nil {
		t.Fatalf("Failed to set raw.idmap: %v", err)
	}

	// Create a temp dir with a host-owned file to mount as workspace
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/host-file.txt", []byte("written-by-host"), 0o644); err != nil {
		t.Fatalf("Failed to create host file: %v", err)
	}

	// Mount workspace WITHOUT shift (raw.idmap handles UID mapping)
	if err := mgr.MountDisk("workspace", tmpDir, "/workspace", false, false); err != nil {
		t.Fatalf("Failed to mount workspace: %v", err)
	}

	// Start the container
	if err := mgr.Start(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Test read/write access as code user — should always succeed
	failures := testWorkspaceWriteAccess(t, mgr)
	for _, f := range failures {
		t.Errorf("raw.idmap should fix UID mapping but got failure: %s", f)
	}
}
