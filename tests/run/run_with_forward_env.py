"""
Test for coi run - with --forward-env flag.

Tests that:
1. Run command with --forward-env to forward a host env var by name
2. Verify env var value from host is available in container
"""

import os
import subprocess


def test_run_with_forward_env(coi_binary, cleanup_containers, workspace_dir):
    """
    Test running command with --forward-env flag.

    Flow:
    1. Set a host env var
    2. Run coi run --forward-env VAR_NAME -- sh -c 'echo $VAR_NAME'
    3. Verify the host value appears in container output
    """
    env = os.environ.copy()
    env["COI_TEST_FORWARD_VAR"] = "forwarded-value-42"

    result = subprocess.run(
        [
            coi_binary,
            "run",
            "--workspace",
            workspace_dir,
            "--forward-env",
            "COI_TEST_FORWARD_VAR",
            "--",
            "sh",
            "-c",
            "echo $COI_TEST_FORWARD_VAR",
        ],
        capture_output=True,
        text=True,
        timeout=180,
        env=env,
    )

    assert result.returncode == 0, f"Run should succeed. stderr: {result.stderr}"

    combined_output = result.stdout + result.stderr
    assert "forwarded-value-42" in combined_output, (
        f"Output should contain forwarded env var value. Got:\n{combined_output}"
    )
