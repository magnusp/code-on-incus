package tool

import (
	"os"
	"path/filepath"
	"strings"
)

// Tool represents an AI coding tool that can be run in COI containers
type Tool interface {
	// Name returns the tool name (e.g., "claude", "aider", "cursor")
	Name() string

	// Binary returns the binary name to execute
	Binary() string

	// ConfigDirName returns config directory name (e.g., ".claude", ".config/opencode")
	// Return "" if tool uses ENV API keys instead of config files
	ConfigDirName() string

	// SessionsDirName returns the sessions directory name for this tool
	// (e.g., "sessions-claude", "sessions-aider")
	SessionsDirName() string

	// BuildCommand builds the command line for execution
	// sessionID: COI session ID
	// resume: whether to resume an existing session
	// resumeSessionID: the tool's internal session ID (if resuming)
	BuildCommand(sessionID string, resume bool, resumeSessionID string) []string

	// DiscoverSessionID finds the tool's internal session ID from saved state
	// stateDir: path to the tool's config directory with saved state
	// Return "" if tool doesn't support session resume (will start fresh each time)
	DiscoverSessionID(stateDir string) string

	// GetSandboxSettings returns settings to inject for sandbox/bypass permissions
	// Return empty map if tool doesn't need settings injection
	GetSandboxSettings() map[string]interface{}
}

// ToolWithConfigDirFiles is implemented by every tool that uses a config
// directory (ConfigDirName != ""). It tells setupCLIConfig which files to
// copy, where to inject sandbox settings, and whether a sibling state file
// (e.g. ~/.claude.json) exists.
type ToolWithConfigDirFiles interface {
	Tool

	// EssentialConfigFiles returns filenames inside the config directory
	// that should be copied from the host (e.g. ["settings.json", ".credentials.json"]).
	EssentialConfigFiles() []string

	// SandboxSettingsFileName returns the filename inside the config directory
	// where GetSandboxSettings() should be injected (e.g. "settings.json", "opencode.json").
	SandboxSettingsFileName() string

	// StateConfigFileName returns the name of a JSON state file that lives as
	// a sibling next to the config directory (e.g. ".claude.json").
	// Return "" if the tool has no such file.
	StateConfigFileName() string

	// AlwaysSetupConfig returns true if setupCLIConfig should run even when
	// the host config directory doesn't exist (e.g. opencode needs sandbox
	// injection regardless). Return false to skip setup when there's nothing
	// to copy from the host (e.g. Claude needs credentials from ~/.claude).
	AlwaysSetupConfig() bool
}

// ClaudeTool implements Tool for Claude Code
type ClaudeTool struct {
	effortLevel string // "low", "medium", "high" - defaults to "medium" if empty
}

// NewClaude creates a new Claude tool instance
func NewClaude() Tool {
	return &ClaudeTool{}
}

func (c *ClaudeTool) Name() string {
	return "claude"
}

func (c *ClaudeTool) Binary() string {
	return "claude"
}

func (c *ClaudeTool) ConfigDirName() string {
	return ".claude"
}

func (c *ClaudeTool) SessionsDirName() string {
	return "sessions-claude"
}

func (c *ClaudeTool) BuildCommand(sessionID string, resume bool, resumeSessionID string) []string {
	// Base command with flags
	cmd := []string{"claude", "--verbose", "--permission-mode", "bypassPermissions"}

	// Add session/resume flag
	if resume {
		if resumeSessionID != "" {
			cmd = append(cmd, "--resume", resumeSessionID)
		} else {
			cmd = append(cmd, "--resume")
		}
	} else {
		cmd = append(cmd, "--session-id", sessionID)
	}

	return cmd
}

func (c *ClaudeTool) DiscoverSessionID(stateDir string) string {
	// Claude stores sessions as .jsonl files in projects/-workspace/
	// This logic is extracted from cleanup.go:387-411
	projectsDir := filepath.Join(stateDir, "projects", "-workspace")

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ""
	}

	// Find the first .jsonl file (Claude session file)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			return strings.TrimSuffix(entry.Name(), ".jsonl")
		}
	}

	return ""
}

func (c *ClaudeTool) GetSandboxSettings() map[string]interface{} {
	// Settings to inject into .claude/settings.json for bypassing permissions
	// and setting effort level
	settings := map[string]interface{}{
		"allowDangerouslySkipPermissions": true,
		"bypassPermissionsModeAccepted":   true,
		"permissions": map[string]string{
			"defaultMode": "bypassPermissions",
		},
	}

	// Set effort level (default to "medium" if not configured)
	// We set both effortLevel directly AND via env var for maximum compatibility
	effortLevel := c.effortLevel
	if effortLevel == "" {
		effortLevel = "medium"
	}
	settings["effortLevel"] = effortLevel

	// Effort prompt suppression flags (discovered by examining .claude.json after manual selection)
	settings["effortLevelAccepted"] = true
	settings["hasSeenEffortPrompt"] = true
	settings["effortCalloutDismissed"] = true

	// Also set via env section (CLAUDE_CODE_EFFORT_LEVEL) as documented
	settings["env"] = map[string]string{
		"CLAUDE_CODE_EFFORT_LEVEL": effortLevel,
	}

	return settings
}

// EssentialConfigFiles implements ToolWithConfigDirFiles.
func (c *ClaudeTool) EssentialConfigFiles() []string {
	return []string{".credentials.json", "config.yml", "settings.json"}
}

// SandboxSettingsFileName implements ToolWithConfigDirFiles.
func (c *ClaudeTool) SandboxSettingsFileName() string { return "settings.json" }

// StateConfigFileName implements ToolWithConfigDirFiles.
// Claude uses ~/.claude.json as a sibling state file next to ~/.claude/.
func (c *ClaudeTool) StateConfigFileName() string { return ".claude.json" }

// AlwaysSetupConfig implements ToolWithConfigDirFiles.
// Claude needs credentials from ~/.claude, so skip setup when host dir is missing.
func (c *ClaudeTool) AlwaysSetupConfig() bool { return false }

// SetEffortLevel sets the effort level for Claude Code.
// Valid values: "low", "medium", "high".
// This prevents the interactive effort prompt during autonomous sessions.
func (c *ClaudeTool) SetEffortLevel(level string) {
	c.effortLevel = level
}

// ToolWithEffortLevel is an optional interface for tools that support
// configurable effort levels (e.g., Claude's low/medium/high effort).
type ToolWithEffortLevel interface {
	Tool
	// SetEffortLevel sets the effort level for the tool.
	// Valid values depend on the tool (e.g., "low", "medium", "high" for Claude).
	SetEffortLevel(level string)
}
