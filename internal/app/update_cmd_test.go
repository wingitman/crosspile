package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWindowsDetachedUpdateCmdPullAndInstall(t *testing.T) {
	repoDir := makeWindowsUpdateRepo(t)
	cmd := buildWindowsDetachedUpdateCmd(repoDir, true, 12345)
	joined := strings.Join(cmd.Args, "\n")
	for _, want := range []string{"powershell", "Start-Process powershell", "Wait-Process -Id 12345", "GCM_INTERACTIVE", "git -C", " pull", "install.ps1", "NoExit"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command missing %q in:\n%s", want, joined)
		}
	}
}

func TestBuildUnixDetachedUpdateCmdPullAndInstall(t *testing.T) {
	repoDir := makeUnixUpdateRepo(t)
	cmd := buildUnixDetachedUpdateCmd(repoDir, true, 12345)
	joined := strings.Join(cmd.Args, "\n")
	for _, want := range []string{"nohup sh -c", "kill -0 12345", "GCM_INTERACTIVE=never git -C", " pull", "make -C", " install", "crosspile-update.log"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command missing %q in:\n%s", want, joined)
		}
	}
}

func makeWindowsUpdateRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install.ps1"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeUnixUpdateRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("install:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
