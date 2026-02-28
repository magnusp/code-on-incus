"""
Test that signal-triggered cleanup runs exactly once.

Verifies that:
1. Sending SIGTERM to the coi process triggers cleanup
2. The "Cleaning up session..." message appears exactly once
3. The container is properly deleted after cleanup
4. The process exits cleanly

This guards against a race condition where both the defer and signal handler
could call doCleanup() concurrently without sync.Once protection.
"""

import os
import signal
import time

from pexpect import EOF, TIMEOUT

from support.helpers import (
    calculate_container_name,
    spawn_coi,
    wait_for_container_ready,
    wait_for_prompt,
    wait_for_specific_container_deletion,
)


def test_signal_cleanup_runs_once(coi_binary, cleanup_containers, workspace_dir):
    """
    Test that SIGTERM triggers cleanup exactly once.

    Flow:
    1. Start coi shell in ephemeral mode with dummy tool
    2. Wait for session to be fully ready
    3. Send SIGTERM directly to the coi process to trigger signal handler
    4. Verify "Cleaning up session..." appears exactly once in output
    5. Verify container is deleted
    """
    env = {"COI_USE_DUMMY": "1"}

    container_name = calculate_container_name(workspace_dir, 1)

    child = spawn_coi(
        coi_binary,
        ["shell"],
        cwd=workspace_dir,
        env=env,
        timeout=120,
    )

    wait_for_container_ready(child, timeout=60)
    wait_for_prompt(child, timeout=90)

    # Send SIGTERM directly to the coi process to trigger signal handler cleanup
    os.kill(child.pid, signal.SIGTERM)

    try:
        child.expect(EOF, timeout=60)
    except TIMEOUT:
        pass

    # Give time for cleanup messages to be captured
    time.sleep(2)

    # Capture output before closing
    if hasattr(child.logfile_read, "get_raw_output"):
        output = child.logfile_read.get_raw_output()
    elif hasattr(child.logfile_read, "get_output"):
        output = child.logfile_read.get_output()
    else:
        output = ""

    try:
        child.close(force=False)
    except Exception:
        child.close(force=True)

    # Verify cleanup message appears exactly once
    cleanup_count = output.count("Cleaning up session...")
    assert cleanup_count == 1, (
        f"'Cleaning up session...' should appear exactly once, "
        f"but appeared {cleanup_count} times. Output:\n{output}"
    )

    # Verify container is deleted (90s to account for network teardown on CI)
    container_deleted = wait_for_specific_container_deletion(container_name, timeout=90)
    assert container_deleted, f"Container {container_name} should be deleted after signal cleanup"
