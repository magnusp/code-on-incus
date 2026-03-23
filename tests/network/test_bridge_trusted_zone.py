"""
Test for bridge firewalld trusted zone detection.

Tests that:
1. BridgeInTrustedZone detection works correctly via the health check
2. The health check details accurately reflect the actual zone state
3. Removing and re-adding the bridge to the trusted zone is detected
"""

import json
import subprocess


def firewalld_available():
    """Check if firewalld is available and running."""
    try:
        result = subprocess.run(
            ["sudo", "-n", "firewall-cmd", "--state"],
            capture_output=True,
            timeout=10,
        )
        return result.returncode == 0
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False


def get_bridge_name():
    """Get the Incus bridge name."""
    try:
        result = subprocess.run(
            ["incus", "network", "list", "-f", "csv"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode == 0:
            for line in result.stdout.strip().split("\n"):
                parts = line.split(",")
                if len(parts) >= 2 and "bridge" in parts[1]:
                    return parts[0]
    except (subprocess.TimeoutExpired, FileNotFoundError):
        pass
    return "incusbr0"


def bridge_in_trusted_zone(bridge_name):
    """Check if the bridge is in the firewalld trusted zone."""
    try:
        result = subprocess.run(
            [
                "sudo",
                "-n",
                "firewall-cmd",
                "--zone=trusted",
                f"--query-interface={bridge_name}",
            ],
            capture_output=True,
            timeout=10,
        )
        return result.returncode == 0
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False


def get_health_bridge_check(coi_binary):
    """Run coi health and return the bridge_firewalld_zone check."""
    result = subprocess.run(
        [coi_binary, "health", "--format", "json"],
        capture_output=True,
        text=True,
        timeout=120,
    )
    data = json.loads(result.stdout)
    return data["checks"].get("bridge_firewalld_zone")


def test_bridge_trusted_zone_detection_matches_reality(coi_binary):
    """
    Test that the health check accurately reflects the actual bridge zone state.

    Flow:
    1. Skip if firewalld not available
    2. Check actual bridge zone state via firewall-cmd
    3. Run coi health and get bridge_firewalld_zone check
    4. Verify the check details match the actual state
    """
    import pytest

    if not firewalld_available():
        pytest.skip("firewalld not available")

    bridge_name = get_bridge_name()
    actual_in_zone = bridge_in_trusted_zone(bridge_name)

    check = get_health_bridge_check(coi_binary)
    assert check is not None, "bridge_firewalld_zone check should exist"
    assert "details" in check, "Check should have details"
    assert check["details"]["in_trusted_zone"] == actual_in_zone, (
        f"Health check reports in_trusted_zone={check['details']['in_trusted_zone']} "
        f"but actual state is {actual_in_zone}"
    )

    if actual_in_zone:
        assert check["status"] == "ok", (
            f"Expected OK when bridge in trusted zone, got: {check['status']}"
        )
    else:
        assert check["status"] == "warning", (
            f"Expected warning when bridge not in trusted zone, got: {check['status']}"
        )


def test_bridge_zone_removal_and_restore_detected(coi_binary):
    """
    Test that removing and re-adding the bridge to trusted zone is properly detected.

    Flow:
    1. Skip if firewalld not available
    2. Record initial state
    3. Remove bridge from trusted zone
    4. Verify health check detects the removal (warning)
    5. Re-add bridge to trusted zone
    6. Verify health check detects the restoration (ok)
    """
    import pytest

    if not firewalld_available():
        pytest.skip("firewalld not available")

    bridge_name = get_bridge_name()
    was_in_zone = bridge_in_trusted_zone(bridge_name)

    try:
        # Step 1: Remove from trusted zone
        subprocess.run(
            [
                "sudo",
                "-n",
                "firewall-cmd",
                "--zone=trusted",
                f"--remove-interface={bridge_name}",
            ],
            capture_output=True,
            timeout=10,
        )

        if bridge_in_trusted_zone(bridge_name):
            pytest.skip("Could not remove bridge from trusted zone")

        # Verify health check detects removal
        check = get_health_bridge_check(coi_binary)
        assert check is not None, "bridge_firewalld_zone check should exist"
        assert check["status"] == "warning", (
            f"Expected warning after removing bridge from trusted zone, got: {check['status']}"
        )
        assert check["details"]["in_trusted_zone"] is False, (
            "Should report in_trusted_zone=false after removal"
        )

        # Step 2: Re-add to trusted zone
        subprocess.run(
            [
                "sudo",
                "-n",
                "firewall-cmd",
                "--zone=trusted",
                f"--add-interface={bridge_name}",
            ],
            capture_output=True,
            timeout=10,
        )

        if not bridge_in_trusted_zone(bridge_name):
            pytest.skip("Could not re-add bridge to trusted zone")

        # Verify health check detects restoration
        check = get_health_bridge_check(coi_binary)
        assert check is not None, "bridge_firewalld_zone check should exist"
        assert check["status"] == "ok", (
            f"Expected OK after restoring bridge to trusted zone, got: {check['status']}"
        )
        assert check["details"]["in_trusted_zone"] is True, (
            "Should report in_trusted_zone=true after restoration"
        )

    finally:
        # Restore original state
        if was_in_zone:
            subprocess.run(
                [
                    "sudo",
                    "-n",
                    "firewall-cmd",
                    "--zone=trusted",
                    f"--add-interface={bridge_name}",
                ],
                capture_output=True,
                timeout=10,
            )
        else:
            subprocess.run(
                [
                    "sudo",
                    "-n",
                    "firewall-cmd",
                    "--zone=trusted",
                    f"--remove-interface={bridge_name}",
                ],
                capture_output=True,
                timeout=10,
            )
