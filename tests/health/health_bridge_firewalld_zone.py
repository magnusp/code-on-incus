"""
Test for coi health - bridge firewalld zone check.

Tests that:
1. The bridge_firewalld_zone check exists in health output
2. When firewalld is running and bridge is in trusted zone, check is OK
3. When bridge is not in trusted zone, check warns with fix command
4. Check appears in text output under NETWORKING section
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


def test_health_bridge_firewalld_zone_check_exists(coi_binary):
    """
    Verify bridge_firewalld_zone check appears in health JSON output.

    Flow:
    1. Run coi health --format json
    2. Parse JSON, verify bridge_firewalld_zone key exists
    3. Verify it has required fields
    """
    result = subprocess.run(
        [coi_binary, "health", "--format", "json"],
        capture_output=True,
        text=True,
        timeout=120,
    )

    assert result.returncode in [0, 1, 2], (
        f"Health check failed unexpectedly. stderr: {result.stderr}"
    )

    data = json.loads(result.stdout)
    checks = data["checks"]

    assert "bridge_firewalld_zone" in checks, (
        "Should have bridge_firewalld_zone check"
    )

    check = checks["bridge_firewalld_zone"]
    assert "name" in check, "Check should have 'name' field"
    assert "status" in check, "Check should have 'status' field"
    assert "message" in check, "Check should have 'message' field"
    assert check["status"] in ["ok", "warning", "failed"], (
        f"Invalid status: {check['status']}"
    )


def test_health_bridge_firewalld_zone_ok_when_configured(coi_binary):
    """
    When firewalld is running and bridge is in trusted zone, check reports OK.

    Flow:
    1. Skip if firewalld not available or bridge not in trusted zone
    2. Run coi health --format json
    3. Verify bridge_firewalld_zone check is OK with correct details
    """
    import pytest

    if not firewalld_available():
        pytest.skip("firewalld not available")

    bridge_name = get_bridge_name()
    if not bridge_in_trusted_zone(bridge_name):
        pytest.skip(
            f"Bridge {bridge_name} not in trusted zone (test requires it to be)"
        )

    result = subprocess.run(
        [coi_binary, "health", "--format", "json"],
        capture_output=True,
        text=True,
        timeout=120,
    )

    data = json.loads(result.stdout)
    check = data["checks"]["bridge_firewalld_zone"]

    assert check["status"] == "ok", (
        f"Expected OK when bridge is in trusted zone, got: {check['status']} - {check['message']}"
    )
    assert "trusted zone" in check["message"], (
        f"Message should mention trusted zone: {check['message']}"
    )
    assert "details" in check, "Should have details when firewalld is running"
    assert check["details"]["bridge_name"] == bridge_name, (
        f"Details should show bridge name {bridge_name}"
    )
    assert check["details"]["in_trusted_zone"] is True, (
        "Details should show in_trusted_zone=true"
    )


def test_health_bridge_firewalld_zone_warns_when_not_configured(coi_binary):
    """
    When bridge is removed from trusted zone, health reports warning with fix command.

    Flow:
    1. Skip if firewalld not available
    2. Remove bridge from trusted zone temporarily
    3. Run coi health --format json
    4. Verify warning status with actionable fix command
    5. Restore bridge to trusted zone
    """
    import pytest

    if not firewalld_available():
        pytest.skip("firewalld not available")

    bridge_name = get_bridge_name()

    # Check if bridge is currently in trusted zone so we know whether to restore
    was_in_zone = bridge_in_trusted_zone(bridge_name)

    try:
        # Remove bridge from trusted zone
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

        # Verify it was actually removed
        if bridge_in_trusted_zone(bridge_name):
            pytest.skip("Could not remove bridge from trusted zone for testing")

        result = subprocess.run(
            [coi_binary, "health", "--format", "json"],
            capture_output=True,
            text=True,
            timeout=120,
        )

        data = json.loads(result.stdout)
        check = data["checks"]["bridge_firewalld_zone"]

        assert check["status"] == "warning", (
            f"Expected warning when bridge not in trusted zone, got: {check['status']} - {check['message']}"
        )
        assert "not in trusted zone" in check["message"], (
            f"Message should explain the issue: {check['message']}"
        )
        assert "firewall-cmd" in check["message"], (
            f"Message should include fix command: {check['message']}"
        )
        assert bridge_name in check["message"], (
            f"Message should include bridge name: {check['message']}"
        )
        assert check["details"]["in_trusted_zone"] is False, (
            "Details should show in_trusted_zone=false"
        )

    finally:
        # Restore bridge to trusted zone if it was there before
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


def test_health_bridge_firewalld_zone_ok_without_firewalld(coi_binary):
    """
    When firewalld is not running, bridge zone check is OK (not applicable).

    Flow:
    1. Skip if firewalld IS available (this test is for when it's not)
    2. Run coi health --format json
    3. Verify bridge_firewalld_zone check is OK with not-applicable message
    """
    import pytest

    if firewalld_available():
        pytest.skip("firewalld is available (test is for when it is not)")

    result = subprocess.run(
        [coi_binary, "health", "--format", "json"],
        capture_output=True,
        text=True,
        timeout=120,
    )

    data = json.loads(result.stdout)
    check = data["checks"]["bridge_firewalld_zone"]

    assert check["status"] == "ok", (
        f"Expected OK when firewalld not running, got: {check['status']}"
    )
    assert "not running" in check["message"].lower() or "not applicable" in check["message"].lower(), (
        f"Message should indicate check is not applicable: {check['message']}"
    )


def test_health_bridge_firewalld_zone_text_output(coi_binary):
    """
    Verify bridge zone check appears in text health output under NETWORKING.

    Flow:
    1. Run coi health (text format)
    2. Verify 'Bridge FW zone' appears in output
    """
    result = subprocess.run(
        [coi_binary, "health"],
        capture_output=True,
        text=True,
        timeout=120,
    )

    assert result.returncode in [0, 1, 2], (
        f"Health check failed unexpectedly. stderr: {result.stderr}"
    )

    output = result.stdout
    assert "Bridge FW zone" in output, (
        f"Should show 'Bridge FW zone' in text output. Got:\n{output}"
    )
