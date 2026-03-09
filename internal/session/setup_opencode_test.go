package session

import (
	"encoding/json"
	"testing"

	"github.com/mensfeld/code-on-incus/internal/tool"
)

// TestBuildJSONFromSettings_OpencodeBypass verifies that opencode's sandbox
// settings (bypass mode) are correctly serialized to JSON.
func TestBuildJSONFromSettings_OpencodeBypass(t *testing.T) {
	oc := tool.NewOpencode()
	settings := oc.GetSandboxSettings()

	jsonStr, err := buildJSONFromSettings(settings)
	if err != nil {
		t.Fatalf("buildJSONFromSettings() error: %v", err)
	}

	// Parse the JSON back and verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify permission key
	perm, ok := parsed["permission"]
	if !ok {
		t.Fatal("JSON missing 'permission' key")
	}
	permMap, ok := perm.(map[string]interface{})
	if !ok {
		t.Fatalf("'permission' is %T, want map", perm)
	}
	if permMap["*"] != "allow" {
		t.Errorf("permission['*'] = %v, want 'allow'", permMap["*"])
	}
}

// TestBuildJSONFromSettings_OpencodeInteractive verifies that interactive mode
// produces correct sandbox settings JSON.
func TestBuildJSONFromSettings_OpencodeInteractive(t *testing.T) {
	oc := &tool.OpencodeTool{}
	oc.SetPermissionMode("interactive")
	settings := oc.GetSandboxSettings()

	jsonStr, err := buildJSONFromSettings(settings)
	if err != nil {
		t.Fatalf("buildJSONFromSettings() error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify permission is "ask" in interactive mode
	permMap := parsed["permission"].(map[string]interface{})
	if permMap["*"] != "ask" {
		t.Errorf("permission['*'] = %v, want 'ask'", permMap["*"])
	}
}

// TestBuildJSONFromSettings_OpencodeRoundTrip verifies that the JSON output
// can round-trip: marshal -> unmarshal -> re-marshal produces identical output.
// This ensures no data is lost when the settings are written to opencode.json.
func TestBuildJSONFromSettings_OpencodeRoundTrip(t *testing.T) {
	oc := tool.NewOpencode()
	settings := oc.GetSandboxSettings()

	jsonStr, err := buildJSONFromSettings(settings)
	if err != nil {
		t.Fatalf("buildJSONFromSettings() error: %v", err)
	}

	// Unmarshal and re-marshal
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Failed to parse first JSON: %v", err)
	}

	jsonStr2, err := buildJSONFromSettings(parsed)
	if err != nil {
		t.Fatalf("second buildJSONFromSettings() error: %v", err)
	}

	if jsonStr != jsonStr2 {
		t.Errorf("Round-trip mismatch:\n  first:  %s\n  second: %s", jsonStr, jsonStr2)
	}
}
