"""
Test for UID mapping fix using raw.idmap instead of shift=true.

Validates that raw.idmap correctly maps the host user's UID to the container's
code user UID (1000), allowing full read/write access regardless of host UID.

This is the fix for the shift=true bug: raw.idmap explicitly maps
"both <hostUID> 1000", so the container's code user always sees host files
as its own.

Tests that:
1. Create container without starting (incus init)
2. Set raw.idmap to map host UID -> container UID 1000
3. Mount workspace WITHOUT shift
4. Start container
5. As code user (UID 1000), read, create, and overwrite files
6. All operations succeed regardless of host UID
"""

import os
import subprocess
import tempfile
import time

from support.helpers import (
    calculate_container_name,
)


def test_workspace_write_access_raw_idmap(coi_binary, cleanup_containers, workspace_dir):
    """
    Test that raw.idmap workspace mounts allow the code user to read/write files.

    Unlike shift=true, raw.idmap explicitly maps the host UID to container
    UID 1000, which works regardless of the host user's UID.

    Flow:
    1. Launch a container (gets Docker/nesting support via coi launch)
    2. Set raw.idmap via incus to map host UID -> 1000
    3. Mount a temp directory WITHOUT shift at /workspace
    4. Exec as code user (UID 1000) to read, create, and overwrite files
    5. Assert all operations succeed
    6. Cleanup
    """
    container_name = calculate_container_name(workspace_dir, 1)
    host_uid = os.getuid()

    # === Phase 1: Create temp directory with a host-owned file ===

    with tempfile.TemporaryDirectory() as tmpdir:
        test_file = os.path.join(tmpdir, "host-file.txt")
        with open(test_file, "w") as f:
            f.write("written-by-host")

        # === Phase 2: Launch container ===

        result = subprocess.run(
            [coi_binary, "container", "launch", "coi", container_name],
            capture_output=True,
            text=True,
            timeout=120,
        )

        assert result.returncode == 0, f"Container launch should succeed. stderr: {result.stderr}"

        time.sleep(3)

        # === Phase 3: Stop container to apply raw.idmap ===
        # raw.idmap requires a container restart to take effect

        result = subprocess.run(
            [coi_binary, "container", "stop", container_name, "--force"],
            capture_output=True,
            text=True,
            timeout=30,
        )

        assert result.returncode == 0, f"Container stop should succeed. stderr: {result.stderr}"

        # === Phase 4: Set raw.idmap and mount workspace ===

        idmap_value = f"both {host_uid} 1000"
        result = subprocess.run(
            [
                "sg",
                "incus-admin",
                "-c",
                f"incus config set {container_name} raw.idmap '{idmap_value}'",
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )

        assert result.returncode == 0, f"Setting raw.idmap should succeed. stderr: {result.stderr}"

        # Mount workspace WITHOUT shift (raw.idmap handles UID mapping)
        result = subprocess.run(
            [
                coi_binary,
                "container",
                "mount",
                container_name,
                "workspace",
                tmpdir,
                "/workspace",
            ],
            capture_output=True,
            text=True,
            timeout=60,
        )

        assert result.returncode == 0, f"Mount should succeed. stderr: {result.stderr}"

        # === Phase 5: Start container with new config ===

        result = subprocess.run(
            [coi_binary, "container", "start", container_name],
            capture_output=True,
            text=True,
            timeout=60,
        )

        assert result.returncode == 0, f"Container start should succeed. stderr: {result.stderr}"

        time.sleep(3)

        # === Phase 6: Test read/write as code user (UID 1000) ===

        # 6a. Read the host-created file
        result = subprocess.run(
            [
                coi_binary,
                "container",
                "exec",
                container_name,
                "--user",
                "1000",
                "--group",
                "1000",
                "--",
                "cat",
                "/workspace/host-file.txt",
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )

        assert result.returncode == 0, (
            f"Code user should be able to read host file with raw.idmap "
            f"(host UID={host_uid} -> container UID=1000). stderr: {result.stderr}"
        )
        assert "written-by-host" in result.stdout + result.stderr, (
            f"Host file should contain expected content. Got: {result.stdout + result.stderr}"
        )

        # 6b. Create a new file
        result = subprocess.run(
            [
                coi_binary,
                "container",
                "exec",
                container_name,
                "--user",
                "1000",
                "--group",
                "1000",
                "--",
                "sh",
                "-c",
                "echo created-by-code > /workspace/code-file.txt",
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )

        assert result.returncode == 0, (
            f"Code user should be able to create files with raw.idmap "
            f"(host UID={host_uid} -> container UID=1000). stderr: {result.stderr}"
        )

        # 6c. Overwrite the host-created file
        result = subprocess.run(
            [
                coi_binary,
                "container",
                "exec",
                container_name,
                "--user",
                "1000",
                "--group",
                "1000",
                "--",
                "sh",
                "-c",
                "echo overwritten-by-code > /workspace/host-file.txt",
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )

        assert result.returncode == 0, (
            f"Code user should be able to overwrite host files with raw.idmap "
            f"(host UID={host_uid} -> container UID=1000). stderr: {result.stderr}"
        )

        # === Phase 7: Cleanup ===

        subprocess.run(
            [coi_binary, "container", "delete", container_name, "--force"],
            capture_output=True,
            timeout=30,
        )
