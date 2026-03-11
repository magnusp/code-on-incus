"""
Test for coi run - with multiple --forward-env variables.

Tests that:
1. Run command with --forward-env forwarding multiple host env vars
2. Verify all forwarded values are available in container
"""

import os
import subprocess


def test_run_with_forward_env_multiple(coi_binary, cleanup_containers, workspace_dir):
    """
    Test running command with multiple forwarded env vars.

    Flow:
    1. Set multiple host env vars
    2. Run coi run --forward-env VAR1,VAR2 -- sh -c 'echo $VAR1 $VAR2'
    3. Verify both values appear in container output
    """
    env = os.environ.copy()
    env["COI_FWD_A"] = "alpha-val"
    env["COI_FWD_B"] = "beta-val"

    result = subprocess.run(
        [
            coi_binary,
            "run",
            "--workspace",
            workspace_dir,
            "--forward-env",
            "COI_FWD_A,COI_FWD_B",
            "--",
            "sh",
            "-c",
            "echo $COI_FWD_A $COI_FWD_B",
        ],
        capture_output=True,
        text=True,
        timeout=180,
        env=env,
    )

    assert result.returncode == 0, f"Run should succeed. stderr: {result.stderr}"

    combined_output = result.stdout + result.stderr
    assert "alpha-val" in combined_output, (
        f"Output should contain COI_FWD_A value. Got:\n{combined_output}"
    )
    assert "beta-val" in combined_output, (
        f"Output should contain COI_FWD_B value. Got:\n{combined_output}"
    )
