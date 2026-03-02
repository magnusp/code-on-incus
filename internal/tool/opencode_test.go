package tool

import (
	"testing"
)

func TestOpencodeTool_Basics(t *testing.T) {
	oc := NewOpencode()

	if oc.Name() != "opencode" {
		t.Errorf("Name() = %q, want %q", oc.Name(), "opencode")
	}
	if oc.Binary() != "opencode" {
		t.Errorf("Binary() = %q, want %q", oc.Binary(), "opencode")
	}
	if oc.ConfigDirName() != ".config/opencode" {
		t.Errorf("ConfigDirName() = %q, want %q", oc.ConfigDirName(), ".config/opencode")
	}
	if oc.SessionsDirName() != "sessions-opencode" {
		t.Errorf("SessionsDirName() = %q, want %q", oc.SessionsDirName(), "sessions-opencode")
	}
}

func TestOpencodeTool_BuildCommand_NewSession(t *testing.T) {
	oc := NewOpencode()
	cmd := oc.BuildCommand("some-session-id", false, "")
	if len(cmd) != 1 || cmd[0] != "opencode" {
		t.Errorf("BuildCommand(new) = %v, want [opencode]", cmd)
	}
}

func TestOpencodeTool_BuildCommand_Resume(t *testing.T) {
	oc := NewOpencode()
	// opencode auto-continues from workspace .opencode/ SQLite, no flag needed
	cmd := oc.BuildCommand("", true, "")
	if len(cmd) != 1 || cmd[0] != "opencode" {
		t.Errorf("BuildCommand(resume) = %v, want [opencode]", cmd)
	}
}

func TestOpencodeTool_BuildCommand_ResumeWithID(t *testing.T) {
	oc := NewOpencode()
	// opencode auto-continues from workspace .opencode/ SQLite, no flag needed
	cmd := oc.BuildCommand("", true, "some-id")
	if len(cmd) != 1 || cmd[0] != "opencode" {
		t.Errorf("BuildCommand(resume with ID) = %v, want [opencode]", cmd)
	}
}

func TestOpencodeTool_DiscoverSessionID(t *testing.T) {
	oc := NewOpencode()
	id := oc.DiscoverSessionID("/some/path")
	if id != "" {
		t.Errorf("DiscoverSessionID() = %q, want %q", id, "")
	}
}

func TestOpencodeTool_GetSandboxSettings(t *testing.T) {
	oc := NewOpencode()
	settings := oc.GetSandboxSettings()

	perm, ok := settings["permission"]
	if !ok {
		t.Fatal("GetSandboxSettings() missing 'permission' key")
	}
	permMap, ok := perm.(map[string]interface{})
	if !ok {
		t.Fatalf("'permission' value is %T, want map[string]interface{}", perm)
	}
	val, ok := permMap["*"]
	if !ok {
		t.Fatal("permission map missing '*' key")
	}
	if val != "allow" {
		t.Errorf("permission['*'] = %q, want %q", val, "allow")
	}
}

func TestOpencodeTool_EssentialConfigFiles(t *testing.T) {
	oc := NewOpencode()
	tcf, ok := oc.(ToolWithConfigDirFiles)
	if !ok {
		t.Fatal("OpencodeTool does not implement ToolWithConfigDirFiles")
	}
	files := tcf.EssentialConfigFiles()
	expected := []string{"opencode.json", "tui.json"}
	if len(files) != len(expected) {
		t.Fatalf("EssentialConfigFiles() = %v, want %v", files, expected)
	}
	for i, f := range files {
		if f != expected[i] {
			t.Errorf("EssentialConfigFiles()[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

func TestOpencodeTool_SandboxSettingsFileName(t *testing.T) {
	oc := NewOpencode()
	tcf, ok := oc.(ToolWithConfigDirFiles)
	if !ok {
		t.Fatal("OpencodeTool does not implement ToolWithConfigDirFiles")
	}
	if tcf.SandboxSettingsFileName() != "opencode.json" {
		t.Errorf("SandboxSettingsFileName() = %q, want %q", tcf.SandboxSettingsFileName(), "opencode.json")
	}
}

func TestOpencodeTool_StateConfigFileName(t *testing.T) {
	oc := NewOpencode()
	tcf, ok := oc.(ToolWithConfigDirFiles)
	if !ok {
		t.Fatal("OpencodeTool does not implement ToolWithConfigDirFiles")
	}
	if tcf.StateConfigFileName() != "" {
		t.Errorf("StateConfigFileName() = %q, want %q", tcf.StateConfigFileName(), "")
	}
}

func TestOpencodeTool_AlwaysSetupConfig(t *testing.T) {
	oc := NewOpencode()
	tcf, ok := oc.(ToolWithConfigDirFiles)
	if !ok {
		t.Fatal("OpencodeTool does not implement ToolWithConfigDirFiles")
	}
	if !tcf.AlwaysSetupConfig() {
		t.Error("AlwaysSetupConfig() = false, want true")
	}
}

func TestOpencodeTool_RegistryLookup(t *testing.T) {
	oc, err := Get("opencode")
	if err != nil {
		t.Fatalf("Get(\"opencode\") returned error: %v", err)
	}
	if oc.Name() != "opencode" {
		t.Errorf("Name() = %q, want %q", oc.Name(), "opencode")
	}
}

func TestOpencodeTool_ImplementsPermissionMode(t *testing.T) {
	oc := NewOpencode()

	twpm, ok := oc.(ToolWithPermissionMode)
	if !ok {
		t.Fatal("OpencodeTool should implement ToolWithPermissionMode")
	}

	// Verify method works without panic
	twpm.SetPermissionMode("interactive")
}

func TestOpencodeTool_GetSandboxSettings_InteractiveMode(t *testing.T) {
	oc := &OpencodeTool{permissionMode: "interactive"}
	settings := oc.GetSandboxSettings()

	if len(settings) != 0 {
		t.Errorf("Expected empty map in interactive mode, got %v", settings)
	}
}

func TestOpencodeTool_GetSandboxSettings_BypassDefault(t *testing.T) {
	oc := &OpencodeTool{} // Empty permissionMode = default bypass
	settings := oc.GetSandboxSettings()

	perm, ok := settings["permission"]
	if !ok {
		t.Fatal("GetSandboxSettings() missing 'permission' key in bypass mode")
	}
	permMap, ok := perm.(map[string]interface{})
	if !ok {
		t.Fatalf("'permission' value is %T, want map[string]interface{}", perm)
	}
	val, ok := permMap["*"]
	if !ok {
		t.Fatal("permission map missing '*' key")
	}
	if val != "allow" {
		t.Errorf("permission['*'] = %q, want %q", val, "allow")
	}
}

func TestListSupported_IncludesOpencode(t *testing.T) {
	supported := ListSupported()
	found := false
	for _, name := range supported {
		if name == "opencode" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListSupported() = %v, does not include 'opencode'", supported)
	}
}
