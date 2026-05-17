package updatecheck

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wingitman/crosspile/internal/config"
	"github.com/wingitman/crosspile/internal/updater"
)

var gitOutputFunc = gitOutput
var execCommand = exec.Command
var remoteRepoCheckFunc = updater.CheckRepo
var remoteURLCheckFunc = updater.CheckRemoteURL

type ResultKind int

const (
	UpToDate ResultKind = iota
	LocalRepoAhead
	RemoteAhead
	MetadataMissing
	RepoMissing
	CheckUnavailable
)

type BakedInfo struct {
	Origin  string
	Version string
	RepoDir string
}

type InstallState struct {
	RepoDir         string
	Origin          string
	InstalledCommit string
	LocalRepoCommit string
	RemoteCommit    string
	Source          string
	CanPull         bool
	CanInstall      bool
}

type Result struct {
	Kind          ResultKind
	State         InstallState
	Message       string
	RecentChanges []string
	Err           error
}

func StartupCheck(baked BakedInfo) Result {
	state := ResolveState(baked)
	if state.InstalledCommit == "" || state.InstalledCommit == "dev" || state.InstalledCommit == "unknown" {
		return Result{Kind: MetadataMissing, State: state, Message: "Install metadata is incomplete. Re-run installer now?"}
	}
	if state.RepoDir == "" {
		return Result{Kind: RepoMissing, State: state, Message: "Install repository was not found. Re-run installer now?"}
	}
	if state.LocalRepoCommit != "" && !commitsMatch(state.LocalRepoCommit, state.InstalledCommit) {
		state.CanInstall = hasInstaller(state.RepoDir)
		return Result{
			Kind:          LocalRepoAhead,
			State:         state,
			Message:       "Your local repo has a newer version than the installed binary. Reinstall now?",
			RecentChanges: recentLocalChanges(state.RepoDir, state.InstalledCommit, state.LocalRepoCommit, 3),
		}
	}
	var available bool
	var latest string
	var err error
	if state.CanPull {
		available, latest, err = remoteRepoCheckFunc(state.RepoDir, state.InstalledCommit)
	} else {
		available, latest, err = remoteURLCheckFunc(state.Origin, state.InstalledCommit)
	}
	if err != nil {
		if errors.Is(err, updater.ErrCheckUnavailable) {
			return Result{Kind: CheckUnavailable, State: state, Err: err}
		}
		if errors.Is(err, updater.ErrVersionUnknown) {
			state.RemoteCommit = latest
			return Result{Kind: MetadataMissing, State: state, Message: "Install metadata is incomplete. Re-run installer now?", Err: err}
		}
		return Result{Kind: CheckUnavailable, State: state, Err: err}
	}
	if available {
		state.RemoteCommit = latest
		state.CanPull = isGitRepo(state.RepoDir)
		return Result{
			Kind:          RemoteAhead,
			State:         state,
			Message:       "A newer remote version is available. Pull and reinstall now?",
			RecentChanges: recentRemoteChanges(state.RepoDir, state.InstalledCommit, latest, 3),
		}
	}
	return Result{Kind: UpToDate, State: state}
}

func ResolveState(baked BakedInfo) InstallState {
	meta := config.ReadInstallMeta()
	state := InstallState{
		RepoDir:         firstNonEmpty(meta.RepoDir, baked.RepoDir),
		Origin:          firstNonEmpty(meta.Origin, baked.Origin, updater.DefaultRemoteURL),
		InstalledCommit: firstNonEmpty(meta.CurrentCommit, baked.Version),
		Source:          "metadata",
	}
	if meta.RepoDir == "" && meta.CurrentCommit == "" && meta.Origin == "" {
		state.Source = "ldflags"
	}
	if state.RepoDir != "" {
		if info, err := os.Stat(state.RepoDir); err != nil || !info.IsDir() {
			state.RepoDir = ""
		}
	}
	if state.RepoDir != "" && isGitRepo(state.RepoDir) {
		state.LocalRepoCommit = gitOutputFunc("git", "-C", state.RepoDir, "rev-parse", "HEAD")
		if origin := gitOutputFunc("git", "-C", state.RepoDir, "remote", "get-url", "origin"); origin != "" {
			state.Origin = origin
		}
		state.CanPull = true
	}
	if state.RepoDir != "" {
		state.CanInstall = hasInstaller(state.RepoDir)
	}
	return state
}

func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func hasInstaller(dir string) bool {
	if runtime.GOOS == "windows" {
		_, err := os.Stat(filepath.Join(dir, "install.ps1"))
		return err == nil
	}
	_, err := os.Stat(filepath.Join(dir, "Makefile"))
	return err == nil
}

func gitOutput(name string, args ...string) string {
	out, err := execCommand(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func commitsMatch(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

func recentRemoteChanges(repoDir, installedCommit, remoteCommit string, limit int) []string {
	return recentChangesInRange(repoDir, installedCommit, remoteCommit, limit)
}

func recentLocalChanges(repoDir, installedCommit, localCommit string, limit int) []string {
	if changes := recentChangesInRange(repoDir, installedCommit, localCommit, limit); len(changes) > 0 {
		return changes
	}
	return recentChangesAtHead(repoDir, limit)
}

func recentChangesInRange(repoDir, installedCommit, latestCommit string, limit int) []string {
	if repoDir == "" || installedCommit == "" || latestCommit == "" || limit <= 0 || !isGitRepo(repoDir) {
		return nil
	}
	out := gitOutputFunc("git", "-C", repoDir, "log", fmt.Sprintf("--max-count=%d", limit), "--pretty=format:%h %s", installedCommit+".."+latestCommit)
	return parseRecentChanges(out, limit)
}

func recentChangesAtHead(repoDir string, limit int) []string {
	if repoDir == "" || limit <= 0 || !isGitRepo(repoDir) {
		return nil
	}
	out := gitOutputFunc("git", "-C", repoDir, "log", fmt.Sprintf("--max-count=%d", limit), "--pretty=format:%h %s", "HEAD")
	return parseRecentChanges(out, limit)
}

func parseRecentChanges(out string, limit int) []string {
	if out == "" {
		return nil
	}
	var changes []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			changes = append(changes, line)
		}
		if len(changes) == limit {
			break
		}
	}
	return changes
}

func (k ResultKind) String() string {
	switch k {
	case UpToDate:
		return "up-to-date"
	case LocalRepoAhead:
		return "local-repo-ahead"
	case RemoteAhead:
		return "remote-ahead"
	case MetadataMissing:
		return "metadata-missing"
	case RepoMissing:
		return "repo-missing"
	case CheckUnavailable:
		return "check-unavailable"
	default:
		return "unknown"
	}
}
