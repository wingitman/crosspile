package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const DefaultRemoteURL = "https://github.com/wingitman/crosspile.git"

var execCommand = exec.Command

var ErrNotConfigured = errors.New("version information not embedded in binary")
var ErrVersionUnknown = errors.New("binary has no version info; reinstall recommended")
var ErrCheckUnavailable = errors.New("update check unavailable")

func Check(remoteURL, currentCommit string) (bool, string, error) {
	return CheckRemoteURL(remoteURL, currentCommit)
}

func CheckRepo(repoDir, currentCommit string) (bool, string, error) {
	if repoDir == "" {
		return false, "", ErrNotConfigured
	}
	if _, err := runGit("-C", repoDir, "fetch", "origin", "--quiet"); err != nil {
		return false, "", fmt.Errorf("%w: git fetch failed: %v", ErrCheckUnavailable, err)
	}
	latest, err := remoteTrackingCommit(repoDir)
	if err != nil {
		return false, "", fmt.Errorf("%w: git remote tracking ref unavailable: %v", ErrCheckUnavailable, err)
	}
	return compareLatest(latest, currentCommit)
}

func CheckRemoteURL(remoteURL, currentCommit string) (bool, string, error) {
	if remoteURL == "" {
		remoteURL = DefaultRemoteURL
	}
	out, err := runGit("ls-remote", remoteURL, "HEAD")
	if err != nil {
		return false, "", fmt.Errorf("%w: git ls-remote failed: %v", ErrCheckUnavailable, err)
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return false, "", fmt.Errorf("no output from git ls-remote")
	}
	return compareLatest(strings.Fields(line)[0], currentCommit)
}

func remoteTrackingCommit(repoDir string) (string, error) {
	refs := [][]string{
		{"-C", repoDir, "rev-parse", "@{u}"},
		{"-C", repoDir, "rev-parse", "origin/HEAD"},
		{"-C", repoDir, "rev-parse", "origin/main"},
		{"-C", repoDir, "rev-parse", "origin/master"},
	}
	var lastErr error
	for _, args := range refs {
		out, err := runGit(args...)
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
		lastErr = err
	}
	return "", lastErr
}

func runGit(args ...string) ([]byte, error) {
	cmd := execCommand("git", args...)
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(env, "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func compareLatest(latestFull, currentCommit string) (bool, string, error) {
	latestFull = strings.TrimSpace(latestFull)
	short := latestFull
	if len(short) > 7 {
		short = short[:7]
	}
	if currentCommit == "" || currentCommit == "dev" || currentCommit == "unknown" {
		return false, short, ErrVersionUnknown
	}
	if strings.HasPrefix(latestFull, currentCommit) || strings.HasPrefix(currentCommit, latestFull) {
		return false, "", nil
	}
	return true, short, nil
}
