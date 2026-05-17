package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAndReadInstallMeta(t *testing.T) {
	t.Setenv(configHomeEnv(), t.TempDir())
	repoDir := filepath.Join(t.TempDir(), "crosspile")
	if err := WriteInstallMeta(repoDir, "https://example.test/repo", "abc1234", "git", "linux/amd64"); err != nil {
		t.Fatalf("WriteInstallMeta() error = %v", err)
	}
	meta := ReadInstallMeta()
	if meta.RepoDir != repoDir || meta.Origin != "https://example.test/repo" || meta.CurrentCommit != "abc1234" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if meta.InstallSource != "git" || meta.Platform != "linux/amd64" || meta.InstalledAt == "" {
		t.Fatalf("metadata missing install details: %+v", meta)
	}
}

func TestReadInstallMetaBackwardCompatibleRepoDirOnly(t *testing.T) {
	t.Setenv(configHomeEnv(), t.TempDir())
	wantRepoDir := filepath.Join(t.TempDir(), "repo")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, installMetaFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`repo_dir = "`+escapeTomlString(wantRepoDir)+`"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := ReadInstallMeta()
	if meta.RepoDir != wantRepoDir {
		t.Fatalf("RepoDir = %q, want %q", meta.RepoDir, wantRepoDir)
	}
}

func configHomeEnv() string {
	if runtime.GOOS == "windows" {
		return "APPDATA"
	}
	return "XDG_CONFIG_HOME"
}
