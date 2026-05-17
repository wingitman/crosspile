package updater

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestCheckUpToDateWithShortCommit(t *testing.T) {
	argsFile := withFakeGit(t, 0, "abcdef1234567890\tHEAD\n")
	available, latest, err := Check("https://example.test/repo", "abcdef1")
	if err != nil || available || latest != "" {
		t.Fatalf("available=%v latest=%q err=%v", available, latest, err)
	}
	assertFakeGitInvocation(t, argsFile, "ls-remote https://example.test/repo HEAD", true)
}

func TestCheckRemoteAhead(t *testing.T) {
	withFakeGit(t, 0, "1234567890abcdef\tHEAD\n")
	available, latest, err := Check("https://example.test/repo", "abcdef1")
	if err != nil || !available || latest != "1234567" {
		t.Fatalf("available=%v latest=%q err=%v", available, latest, err)
	}
}

func TestCheckVersionUnknown(t *testing.T) {
	withFakeGit(t, 0, "1234567890abcdef\tHEAD\n")
	available, latest, err := Check("https://example.test/repo", "")
	if !errors.Is(err, ErrVersionUnknown) || available || latest != "1234567" {
		t.Fatalf("available=%v latest=%q err=%v", available, latest, err)
	}
}

func TestCheckUnavailable(t *testing.T) {
	withFakeGit(t, 128, "fatal: authentication failed\n")
	_, _, err := Check("https://example.test/repo", "abcdef1")
	if !errors.Is(err, ErrCheckUnavailable) {
		t.Fatalf("err=%v, want ErrCheckUnavailable", err)
	}
}

func withFakeGit(t *testing.T, code int, output string) string {
	t.Helper()
	oldExec := execCommand
	t.Cleanup(func() { execCommand = oldExec })
	argsFile := t.TempDir() + string(os.PathSeparator) + "fake-git.txt"
	execCommand = func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"FAKE_GIT_CODE="+strconv.Itoa(code),
			"FAKE_GIT_OUTPUT="+output,
			"FAKE_GIT_ARGS_FILE="+argsFile,
		)
		return cmd
	}
	return argsFile
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) > 0 {
		args = args[1:]
	}
	if len(args) > 0 {
		args = args[1:]
	}
	if path := os.Getenv("FAKE_GIT_ARGS_FILE"); path != "" {
		content := strings.Join(args, " ") + "\n" + strings.Join(os.Environ(), "\n")
		_ = os.WriteFile(path, []byte(content), 0o644)
	}
	code, _ := strconv.Atoi(os.Getenv("FAKE_GIT_CODE"))
	_, _ = os.Stdout.WriteString(os.Getenv("FAKE_GIT_OUTPUT"))
	os.Exit(code)
}

func assertFakeGitInvocation(t *testing.T, argsFile, wantArgs string, wantEnv bool) {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(data), "\n", 2)
	if parts[0] != wantArgs {
		t.Fatalf("git args=%q, want %q", parts[0], wantArgs)
	}
	if wantEnv && (!strings.Contains(string(data), "GIT_TERMINAL_PROMPT=0") || !strings.Contains(string(data), "GCM_INTERACTIVE=never")) {
		t.Fatal("noninteractive git env missing")
	}
}
