package tool

// OpencodeTool implements Tool for opencode (https://opencode.ai)
type OpencodeTool struct{}

// NewOpencode creates a new opencode tool instance
func NewOpencode() Tool { return &OpencodeTool{} }

func (c *OpencodeTool) Name() string { return "opencode" }

func (c *OpencodeTool) Binary() string { return "opencode" }

// ConfigDirName returns the XDG-standard config directory for opencode.
func (c *OpencodeTool) ConfigDirName() string { return ".config/opencode" }

func (c *OpencodeTool) SessionsDirName() string { return "sessions-opencode" }

// BuildCommand builds the opencode launch command.
// opencode stores all sessions in the workspace's .opencode/ SQLite DB.
// It always starts with a new session — use Ctrl+S inside opencode to switch
// to a previous session. There's no CLI flag for auto-resume.
func (c *OpencodeTool) BuildCommand(sessionID string, resume bool, resumeSessionID string) []string {
	return []string{"opencode"}
}

// DiscoverSessionID returns "" because opencode uses SQLite (not JSONL files).
func (c *OpencodeTool) DiscoverSessionID(stateDir string) string { return "" }

// GetSandboxSettings returns the opencode permission bypass config.
// Injected into ~/.config/opencode/opencode.json so opencode runs without interactive prompts.
func (c *OpencodeTool) GetSandboxSettings() map[string]interface{} {
	return map[string]interface{}{
		"permission": map[string]interface{}{
			"*": "allow",
		},
	}
}

// EssentialConfigFiles implements ToolWithConfigDirFiles.
func (c *OpencodeTool) EssentialConfigFiles() []string {
	return []string{"opencode.json", "tui.json"}
}

// SandboxSettingsFileName implements ToolWithConfigDirFiles.
func (c *OpencodeTool) SandboxSettingsFileName() string { return "opencode.json" }
