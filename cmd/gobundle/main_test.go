package main_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var gobundleBinary string

const (
	sharedConfigNoIdentity = `
identity:
entitlements:
  com.apple.security.app-sandbox: true
`
	appConfig = `
info.plist:
  CFBundleIdentifier: com.shared.bundle
  CFBundleDisplayName: My App
`
)

func buildGobundleBinary() (string, string) {
	tmpDir, err := os.MkdirTemp("", "gobundle-test")
	if err != nil {
		panic(err)
	}
	bin := filepath.Join(tmpDir, "gobundle-test-bin")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	return tmpDir, bin
}

func TestMain(m *testing.M) {
	var tmpDir string
	tmpDir, gobundleBinary = buildGobundleBinary()
	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func TestGobundleHelp(t *testing.T) {
	cmd := exec.Command(gobundleBinary, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
			t.Fatalf("gobundle --help failed: %v (stderr: %s)", err, stderr.String())
		}
		if got, want := stderr.String(), "Usage: gobundle --help|run|build|install|... [options]\n"; !strings.Contains(got, want) {
			t.Fatalf("unexpected help output:\nGot:\n%s\nExpected to contain:\n%s", got, want)
		}
	}
	out := stdout.String() + stderr.String()
	if out == "" {
		t.Errorf("expected help output, got empty string")
	}
}

func TestGobundleNoArgs(t *testing.T) {
	cmd := exec.Command(gobundleBinary) // no args -> should behave like "go"
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 2 {
		t.Fatal(err)
	}
	if got, want := len(stdout.String()), 0; got != want {
		t.Errorf("expected no stdout, got: %s", stdout.String())
	}
	if got, want := stderr.String(), "go <command> [arguments]"; !strings.Contains(got, want) {
		t.Errorf("unexpected stderr output:\nGot:\n%s\nExpected to contain:\n%s", got, want)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {

		t.Fatalf("failed to read file: %v", err)
	}
	return data
}

func exists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {

			t.Fatalf("expected file %s to exist, but it does not", path)
		}
		t.Fatalf("error checking file existence: %v", err)
	}
}

func inspectBundle(t *testing.T, bundle, binary string) string {
	t.Helper()
	exists(t, filepath.Join(bundle, "Contents", "Info.plist"))
	exists(t, filepath.Join(bundle, "Contents", "MacOS", binary))
	exists(t, filepath.Join(bundle, "Contents", "Resources", "gobundle.yml"))
	cfg := readFile(t, filepath.Join(bundle, "Contents", "Resources", "gobundle.yml"))
	return string(cfg)
}

func setupConfig(t *testing.T, tmpDir, bundle string) (shared, app, argStr string) {
	sharedCfg := filepath.Join(tmpDir, "gobundle-shared.yaml")
	appCfg := filepath.Join(tmpDir, "gobundle-app.yaml")
	sharedWithBundle := []byte(sharedConfigNoIdentity)
	if bundle != "" {
		sharedWithBundle = append(sharedWithBundle, fmt.Appendf(nil, "bundle: %s\n", bundle)...)
	}
	if err := os.WriteFile(sharedCfg, sharedWithBundle, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appCfg, []byte(appConfig), 0600); err != nil {
		t.Fatal(err)
	}
	var arg [30]byte
	if _, err := rand.Read(arg[:]); err != nil {
		t.Fatal(err)
	}
	argStr = base64.StdEncoding.EncodeToString(arg[:])
	return sharedCfg, appCfg, argStr
}

func TestGoRun(t *testing.T) {
	tmpDir := t.TempDir()
	sharedCfg, appCfg, argStr := setupConfig(t, tmpDir, "")
	cmd := exec.Command(gobundleBinary, "run", "./testdata/example.go", argStr)
	out := runGoBundle(t, cmd, sharedCfg, appCfg)
	t.Logf("gobundle run output:\n%s\n", out)
	if got, want := out, "hello\n"; !strings.Contains(got, want) {
		t.Errorf("unexpected output:\nGot:\n%s\nExpected:\n%s", got, want)
	}
	if got, want := out, argStr; !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q, got:\n%s", want, got)
	}
}

func runGoBundle(t *testing.T, cmd *exec.Cmd, sharedCfg, appCfg string) string {
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env,
		"GOBUNDLE_VERBOSE=yes",
		"GOBUNDLE_SHARED_CONFIG="+sharedCfg,
		"GOBUNDLE_APP_CONFIG="+appCfg,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gobundle build failed: %v %s)", err, string(out))
	}
	return string(out)
}

func runExample(t *testing.T, binary, argStr string) {
	t.Helper()
	cmd := exec.Command(binary, argStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("PATH: %s\n", os.Getenv("PATH"))
		t.Fatalf("example run failed for %v: %v (%s)", binary, err, string(out))
	}
	if got, want := string(out), "hello\n"; !strings.Contains(got, want) {
		t.Errorf("unexpected output:\nGot:\n%s\nExpected:\n%s", got, want)
	}
	if got, want := string(out), argStr; !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q, got:\n%s", want, got)
	}
}

func getSourcePath(t *testing.T) (string, string) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return cwd, filepath.Join(cwd, "testdata", "example.go")
}

func newWorkingDir(t *testing.T) string {
	t.Helper()
	wd := filepath.Join(t.TempDir(), "a-working-dir")
	if err := os.MkdirAll(wd, 0700); err != nil {
		t.Fatalf("failed to create working directory: %v", err)
	}
	return wd
}

func verifySoftlink(t *testing.T, binary, bundle, name string) {
	t.Helper()
	info, err := os.Lstat(binary)
	if err != nil {
		t.Fatalf("failed to stat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink", binary)
	}
	dest, err := os.Readlink(binary)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if dest != filepath.Join(bundle, "Contents", "MacOS", name) {
		t.Fatalf("expected symlink to point to %s, got %s", bundle, dest)
	}
}

func TestGoBuild(t *testing.T) {
	tmpDir := t.TempDir()

	cwd, srcPath := getSourcePath(t)

	// Test with -o
	sharedCfg, appCfg, argStr := setupConfig(t, tmpDir, "")
	cmd := exec.Command(gobundleBinary, "build", "-o", tmpDir, srcPath)
	out := runGoBundle(t, cmd, sharedCfg, appCfg)
	t.Logf("gobundle build -o output:\n%s\n", out)
	inspectBundle(t, filepath.Join(tmpDir, "example.app"), "example")
	runExample(t, filepath.Join(tmpDir, "example"), argStr)
	verifySoftlink(t, filepath.Join(tmpDir, "example"), filepath.Join(tmpDir, "example.app"), "example")

	// Test with bundle: in the config
	tmpDir = t.TempDir() // avoid conflicts with previous test
	wd := newWorkingDir(t)
	sharedCfg, appCfg, argStr = setupConfig(t, tmpDir, filepath.Join(tmpDir,
		"another-directory/eg.app"))
	cmd = exec.Command(gobundleBinary, "build", srcPath)
	cmd.Dir = wd
	out = runGoBundle(t, cmd, sharedCfg, appCfg)
	t.Logf("gobundle build with bundle: output:\n%s\n", out)
	inspectBundle(t, filepath.Join(tmpDir, "another-directory", "eg.app"), "example")
	runExample(t, filepath.Join(wd, "example"), argStr) // the soflink will be in the current directory
	verifySoftlink(t, filepath.Join(wd, "example"), filepath.Join(tmpDir, "another-directory", "eg.app"), "example")

	// Test without -o or bundle:, the bundle and example will be
	// created in the current directory
	tmpDir = t.TempDir() // avoid conflicts with previous test

	sharedCfg, appCfg, argStr = setupConfig(t, tmpDir, "")
	wd = newWorkingDir(t)
	src := filepath.Join(cwd, "testdata", "example.go")
	cmd = exec.Command(gobundleBinary, "build", src)
	cmd.Dir = wd
	out = runGoBundle(t, cmd, sharedCfg, appCfg)
	t.Logf("gobundle build in current directory output:\n%s\n", out)
	inspectBundle(t, filepath.Join(wd, "example.app"), "example")
	runExample(t, filepath.Join(wd, "example"), argStr)
	verifySoftlink(t, filepath.Join(wd, "example"), "example.app", "example")

}

func TestGoInstall(t *testing.T) {
	tmpDir := t.TempDir()

	sharedCfg, appCfg, argStr := setupConfig(t, tmpDir, "")
	cmd := exec.Command(gobundleBinary, "install", "./testdata/example.go")
	cmd.Env = []string{"HOME=" + tmpDir,
		"PATH=" + os.Getenv("PATH"),
	}
	out := runGoBundle(t, cmd, sharedCfg, appCfg)
	t.Logf("gobundle build -o output:\n%s\n", out)
	inspectBundle(t, filepath.Join(tmpDir, "go", "bin", "example.app"), "example")
	runExample(t, filepath.Join(tmpDir, "go", "bin", "example"), argStr)
	verifySoftlink(t, filepath.Join(tmpDir, "go", "bin", "example"), filepath.Join(tmpDir, "go", "bin", "example.app"), "example")
}
