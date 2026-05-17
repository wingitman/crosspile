package updatecheck

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wingitman/crosspile/internal/config"
)

func TestStartupCheckLocalRepoAheadFromMetadata(t *testing.T) {
	repoDir := makeRepoDir(t)
	writeMeta(t, repoDir, "https://example.test/repo", "old1111")
	stubGitOutput(t, map[string]string{
		"-C " + repoDir + " rev-parse HEAD":                                                   "new222233334444",
		"-C " + repoDir + " remote get-url origin":                                            "https://example.test/repo",
		"-C " + repoDir + " log --max-count=3 --pretty=format:%h %s old1111..new222233334444": "new2222 Add update flow",
	})
	stubRemoteRepoCheck(t, func(repoDirArg, current string) (bool, string, error) {
		t.Fatal("remote check should not run when local repo is ahead")
		return false, "", nil
	})

	result := StartupCheck(BakedInfo{})
	if result.Kind != LocalRepoAhead {
		t.Fatalf("Kind = %s, want %s", result.Kind, LocalRepoAhead)
	}
	if len(result.RecentChanges) != 1 || result.RecentChanges[0] != "new2222 Add update flow" {
		t.Fatalf("RecentChanges = %#v", result.RecentChanges)
	}
}

func TestStartupCheckRemoteAhead(t *testing.T) {
	repoDir := makeRepoDir(t)
	writeMeta(t, repoDir, "https://example.test/repo", "abc1234")
	stubGitOutput(t, map[string]string{
		"-C " + repoDir + " rev-parse HEAD":                                           "abc123456789",
		"-C " + repoDir + " remote get-url origin":                                    "https://example.test/repo",
		"-C " + repoDir + " log --max-count=3 --pretty=format:%h %s abc1234..def5678": "def5678 Improve updater",
	})
	stubRemoteRepoCheck(t, func(repoDirArg, current string) (bool, string, error) {
		return true, "def5678", nil
	})

	result := StartupCheck(BakedInfo{})
	if result.Kind != RemoteAhead || result.State.RemoteCommit != "def5678" {
		t.Fatalf("result = %+v", result)
	}
}

func makeRepoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	installer := "Makefile"
	if runtime.GOOS == "windows" {
		installer = "install.ps1"
	}
	if err := os.WriteFile(filepath.Join(dir, installer), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeMeta(t *testing.T, repoDir, origin, commit string) {
	t.Helper()
	t.Setenv(configHomeEnv(), t.TempDir())
	if err := config.WriteInstallMeta(repoDir, origin, commit, "git", Platform()); err != nil {
		t.Fatal(err)
	}
}

func stubGitOutput(t *testing.T, outputs map[string]string) {
	t.Helper()
	old := gitOutputFunc
	t.Cleanup(func() { gitOutputFunc = old })
	gitOutputFunc = func(name string, args ...string) string {
		return outputs[joinArgs(args)]
	}
}

func stubRemoteRepoCheck(t *testing.T, fn func(repoDir, current string) (bool, string, error)) {
	t.Helper()
	oldRepo := remoteRepoCheckFunc
	oldURL := remoteURLCheckFunc
	t.Cleanup(func() { remoteRepoCheckFunc = oldRepo; remoteURLCheckFunc = oldURL })
	remoteRepoCheckFunc = fn
	remoteURLCheckFunc = func(remote, current string) (bool, string, error) {
		t.Fatal("remote URL check should not run when repo is available")
		return false, "", nil
	}
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func configHomeEnv() string {
	if runtime.GOOS == "windows" {
		return "APPDATA"
	}
	return "XDG_CONFIG_HOME"
}
