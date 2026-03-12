package tome

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const tomeMarker = "# managed by tome"

const postCommitHook = `#!/bin/sh
# managed by tome
if command -v tome >/dev/null 2>&1; then
  tome checkpoint 2>/dev/null &
fi
`

const prePushHook = `#!/bin/sh
# managed by tome
if command -v tome >/dev/null 2>&1; then
  tome sync --push 2>/dev/null || true
fi
`

// InstallHooks installs post-commit and pre-push git hooks for automatic
// transcript capture and sync. Idempotent — skips if marker already present.
// Preserves existing hook content by appending.
func InstallHooks(repoDir string) error {
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", repoDir)
	}

	if err := installHook(hooksDir, "post-commit", postCommitHook); err != nil {
		return fmt.Errorf("install post-commit hook: %w", err)
	}
	if err := installHook(hooksDir, "pre-push", prePushHook); err != nil {
		return fmt.Errorf("install pre-push hook: %w", err)
	}
	return nil
}

// AddGitignore ensures .tome is listed in the repo's .gitignore.
// Idempotent — skips if already present.
func AddGitignore(repoDir string) error {
	gitignorePath := filepath.Join(repoDir, ".gitignore")

	existing, err := os.ReadFile(gitignorePath)
	if err == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			if strings.TrimSpace(line) == ".tome" || strings.TrimSpace(line) == ".tome/" {
				return nil // already present
			}
		}

		// Append .tome to existing .gitignore.
		content := string(existing)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += ".tome/\n"
		return os.WriteFile(gitignorePath, []byte(content), 0o644)
	}

	// No .gitignore — create one.
	return os.WriteFile(gitignorePath, []byte(".tome/\n"), 0o644)
}

// UninstallHooks removes tome-managed sections from git hooks.
// If the hook file only contains tome content, the file is removed entirely.
func UninstallHooks(repoDir string) error {
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil // no hooks dir, nothing to do
	}

	for _, name := range []string{"post-commit", "pre-push"} {
		if err := uninstallHook(hooksDir, name); err != nil {
			return fmt.Errorf("uninstall %s hook: %w", name, err)
		}
	}
	return nil
}

// RemoveGitignore removes the .tome/ entry from .gitignore.
// Removes the file entirely if .tome/ was the only entry.
func RemoveGitignore(repoDir string) error {
	gitignorePath := filepath.Join(repoDir, ".gitignore")

	existing, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil // no .gitignore, nothing to do
	}

	lines := strings.Split(string(existing), "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == ".tome" || trimmed == ".tome/" {
			continue
		}
		filtered = append(filtered, line)
	}

	// If only empty lines remain, remove the file.
	content := strings.Join(filtered, "\n")
	if strings.TrimSpace(content) == "" {
		return os.Remove(gitignorePath)
	}

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}

func uninstallHook(hooksDir, name string) error {
	hookPath := filepath.Join(hooksDir, name)

	existing, err := os.ReadFile(hookPath)
	if err != nil {
		return nil // hook doesn't exist
	}

	content := string(existing)
	if !strings.Contains(content, tomeMarker) {
		return nil // no tome content
	}

	// Remove the tome-managed block. Split into lines and remove everything
	// from the marker line through the end of the tome block.
	lines := strings.Split(content, "\n")
	var filtered []string
	inTomeBlock := false
	for _, line := range lines {
		if strings.Contains(line, tomeMarker) {
			inTomeBlock = true
			continue
		}
		if inTomeBlock {
			// Tome blocks end at the next empty line or shebang.
			// The block includes: marker, if/fi block, trailing empty lines.
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue // skip blank lines within/after tome block
			}
			if strings.HasPrefix(trimmed, "#!") || (!strings.HasPrefix(trimmed, "if ") && !strings.HasPrefix(trimmed, "tome ") && trimmed != "fi") {
				// Non-tome content — stop removing.
				inTomeBlock = false
				filtered = append(filtered, line)
			}
			// else: still in tome block, skip line
			continue
		}
		filtered = append(filtered, line)
	}

	remaining := strings.Join(filtered, "\n")
	remaining = strings.TrimRight(remaining, "\n")

	// If only a shebang (or nothing) remains, remove the file entirely.
	trimmed := strings.TrimSpace(remaining)
	if trimmed == "" || trimmed == "#!/bin/sh" || trimmed == "#!/bin/bash" {
		return os.Remove(hookPath)
	}

	return os.WriteFile(hookPath, []byte(remaining+"\n"), 0o755)
}

func installHook(hooksDir, name, content string) error {
	hookPath := filepath.Join(hooksDir, name)

	existing, err := os.ReadFile(hookPath)
	if err == nil {
		// Hook file exists — check for marker.
		if strings.Contains(string(existing), tomeMarker) {
			return nil // already installed
		}

		// Append tome hook to existing content.
		combined := string(existing)
		if !strings.HasSuffix(combined, "\n") {
			combined += "\n"
		}
		combined += "\n" + content
		return os.WriteFile(hookPath, []byte(combined), 0o755)
	}

	// No existing hook — create new.
	return os.WriteFile(hookPath, []byte(content), 0o755)
}
