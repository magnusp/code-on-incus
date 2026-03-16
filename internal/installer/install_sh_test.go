package installer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installShPath returns the absolute path to install.sh at the repo root.
func installShPath(t *testing.T) string {
	t.Helper()
	// Walk up from this test file to repo root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Join(wd, "..", "..")
	path := filepath.Join(root, "install.sh")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("install.sh not found at %s: %v", path, err)
	}
	return path
}

// runBashSnippet runs a bash snippet that sources the install.sh functions and
// returns stdout, stderr, and the exit code. The snippet runs with
// NONINTERACTIVE=1 by default unless overridden via env.
func runBashSnippet(t *testing.T, snippet string, env ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command("bash", "-c", snippet)
	cmd.Env = append(os.Environ(), env...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run snippet: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// When stdin is not a terminal (piped input), install.sh should detect non-interactive
// mode automatically via the `! [ -t 0 ]` check in the NONINTERACTIVE detection block.
func TestInstallSh_DetectsNonInteractiveWhenPiped(t *testing.T) {
	script := installShPath(t)

	// Source install.sh functions and check the NONINTERACTIVE variable.
	// We pipe through bash (simulating curl|bash) so stdin is not a TTY.
	snippet := `
		# Source just the variable/function definitions (stop before main runs)
		source <(sed '/^main "\$@"/d; /^trap error_handler ERR/d' "` + script + `")
		echo "NONINTERACTIVE=$NONINTERACTIVE"
	`
	stdout, _, exitCode := runBashSnippet(t, snippet)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "NONINTERACTIVE=1") {
		t.Errorf("expected NONINTERACTIVE=1 when piped, got: %s", strings.TrimSpace(stdout))
	}
}

// When NONINTERACTIVE=1 is set explicitly, prompt_continue should abort with a
// clear message instead of trying to read from the terminal.
func TestInstallSh_PromptContinueAbortsNonInteractive(t *testing.T) {
	script := installShPath(t)

	snippet := `
		export NONINTERACTIVE=1
		source <(sed '/^main "\$@"/d; /^trap error_handler ERR/d' "` + script + `")
		prompt_continue "Continue anyway?"
		echo "SHOULD_NOT_REACH"
	`
	stdout, _, exitCode := runBashSnippet(t, snippet, "NONINTERACTIVE=1")
	if exitCode == 0 {
		t.Errorf("expected non-zero exit code from prompt_continue in non-interactive mode")
	}
	if strings.Contains(stdout, "SHOULD_NOT_REACH") {
		t.Errorf("prompt_continue should have exited, but execution continued")
	}
	if !strings.Contains(stdout, "Non-interactive mode") {
		t.Errorf("expected non-interactive warning message, got: %s", stdout)
	}
}

// When NONINTERACTIVE=1 is set, prompt_choice should return the default value
// without attempting to read from the terminal.
func TestInstallSh_PromptChoiceUsesDefaultNonInteractive(t *testing.T) {
	script := installShPath(t)

	snippet := `
		export NONINTERACTIVE=1
		source <(sed '/^main "\$@"/d; /^trap error_handler ERR/d' "` + script + `")
		prompt_choice "Pick one: " "1"
		echo "REPLY=$REPLY"
	`
	stdout, _, exitCode := runBashSnippet(t, snippet, "NONINTERACTIVE=1")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "REPLY=1") {
		t.Errorf("expected REPLY=1 (default), got: %s", strings.TrimSpace(stdout))
	}
}

// When CI=true is set, install.sh should detect non-interactive mode even
// without piped stdin or explicit NONINTERACTIVE flag.
func TestInstallSh_DetectsCI(t *testing.T) {
	script := installShPath(t)

	snippet := `
		export CI=true
		source <(sed '/^main "\$@"/d; /^trap error_handler ERR/d' "` + script + `")
		echo "NONINTERACTIVE=$NONINTERACTIVE"
	`
	stdout, _, exitCode := runBashSnippet(t, snippet, "CI=true")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "NONINTERACTIVE=1") {
		t.Errorf("expected NONINTERACTIVE=1 with CI=true, got: %s", strings.TrimSpace(stdout))
	}
}

// In non-interactive mode, check_firewalld should complete without hanging
// (it should skip the interactive prompts and return cleanly).
func TestInstallSh_FirewalldAutoSetup_NonInteractive(t *testing.T) {
	script := installShPath(t)

	snippet := `
		export NONINTERACTIVE=1
		source <(sed '/^main "\$@"/d; /^trap error_handler ERR/d' "` + script + `")
		check_firewalld
		echo "COMPLETED"
	`
	stdout, _, exitCode := runBashSnippet(t, snippet, "NONINTERACTIVE=1")
	if exitCode != 0 {
		t.Fatalf("check_firewalld exited with %d in non-interactive mode; stdout: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "COMPLETED") {
		t.Errorf("check_firewalld did not complete in non-interactive mode; stdout: %s", stdout)
	}
	// Should not contain any prompt text
	if strings.Contains(stdout, "[y/N]") {
		t.Errorf("check_firewalld tried to prompt in non-interactive mode; stdout: %s", stdout)
	}
	t.Logf("check_firewalld non-interactive output:\n%s", stdout)
}

// prompt_choice should use a non-default value ("2") when that is passed as default.
func TestInstallSh_PromptChoiceCustomDefault(t *testing.T) {
	script := installShPath(t)

	snippet := `
		export NONINTERACTIVE=1
		source <(sed '/^main "\$@"/d; /^trap error_handler ERR/d' "` + script + `")
		prompt_choice "Pick one: " "2"
		echo "REPLY=$REPLY"
	`
	stdout, _, exitCode := runBashSnippet(t, snippet, "NONINTERACTIVE=1")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "REPLY=2") {
		t.Errorf("expected REPLY=2, got: %s", strings.TrimSpace(stdout))
	}
}
