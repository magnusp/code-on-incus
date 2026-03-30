"""
Test for coi run - fixed timezone written to workspace file.

Tests that:
1. Run with --timezone set to a non-host timezone (Asia/Tokyo)
2. Container writes timezone abbreviation to a file in the mounted workspace
3. Host can read the file and verify correct timezone (JST)
"""

import os
import subprocess


def test_run_timezone_fixed_workspace(coi_binary, cleanup_containers, workspace_dir):
    """
    Test that a fixed timezone is applied in the container and visible
    through workspace files on the host.

    Flow:
    1. Run coi run --timezone Asia/Tokyo to write TZ abbreviation to workspace file
    2. Read the file back on the host and verify JST
    """
    # Write timezone abbreviation to workspace file and verify via stdout
    result = subprocess.run(
        [
            coi_binary,
            "run",
            "--workspace",
            workspace_dir,
            "--timezone",
            "Asia/Tokyo",
            "--",
            "sh",
            "-c",
            "date +%Z > /workspace/tz_test.txt && cat /workspace/tz_test.txt",
        ],
        capture_output=True,
        text=True,
        timeout=180,
    )

    assert result.returncode == 0, f"Run should succeed. stderr: {result.stderr}"

    # Verify via stdout
    tz_abbrev = result.stdout.strip()
    assert tz_abbrev == "JST", (
        f"Timezone abbreviation should be JST for Asia/Tokyo, got: {tz_abbrev}"
    )

    # Verify the file is also readable from the host filesystem
    tz_file = os.path.join(workspace_dir, "tz_test.txt")
    assert os.path.exists(tz_file), f"Timezone file should exist at {tz_file}"

    with open(tz_file) as f:
        file_content = f.read().strip()

    assert file_content == "JST", (
        f"Workspace file should contain JST for Asia/Tokyo, got: {file_content}"
    )
