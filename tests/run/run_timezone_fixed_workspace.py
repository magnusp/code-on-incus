"""
Test for coi run - fixed timezone written to workspace file.

Tests that:
1. Run with --timezone set to a non-host timezone (Asia/Tokyo)
2. Container writes timezone info to a file in the mounted workspace
3. Host can read the file and verify correct timezone abbreviation and offset
"""

import os
import subprocess


def test_run_timezone_fixed_workspace(coi_binary, cleanup_containers, workspace_dir):
    """
    Test that a fixed timezone is applied in the container and visible
    through workspace files on the host.

    Flow:
    1. Run coi run --timezone Asia/Tokyo to write TZ info to a workspace file
    2. Read the file back on the host
    3. Verify JST abbreviation and +0900 offset
    """
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
            "date +%Z > /workspace/tz_test.txt && date '+%z' >> /workspace/tz_test.txt",
        ],
        capture_output=True,
        text=True,
        timeout=180,
    )

    assert result.returncode == 0, f"Run should succeed. stderr: {result.stderr}"

    # Read the file back from the host filesystem
    tz_file = os.path.join(workspace_dir, "tz_test.txt")
    assert os.path.exists(tz_file), f"Timezone file should exist at {tz_file}"

    with open(tz_file) as f:
        lines = f.read().strip().split("\n")

    assert len(lines) >= 2, f"Expected at least 2 lines, got {len(lines)}: {lines}"

    tz_abbrev = lines[0].strip()
    tz_offset = lines[1].strip()

    assert tz_abbrev == "JST", (
        f"Timezone abbreviation should be JST for Asia/Tokyo, got: {tz_abbrev}"
    )
    assert tz_offset == "+0900", (
        f"Timezone offset should be +0900 for Asia/Tokyo, got: {tz_offset}"
    )
