// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"cloudeng.io/macos/machutils"
	"cloudeng.io/os/executil"
)

var testBinary string

func buildTestBinary() (string, string, error) {
	tmpDir, err := os.MkdirTemp("", "machutils-test")
	if err != nil {
		return "", "", err
	}
	bin, err := executil.GoBuild(context.Background(), filepath.Join(tmpDir, "machutils-test-bin"), "./testdata")
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("failed to build test binary: %w", err)
	}
	return tmpDir, bin, nil
}

func TestMain(m *testing.M) {
	tmpDir, bin, err := buildTestBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	testBinary = bin
	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func runSubprocess(t *testing.T, env []string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(testBinary, args...)
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestSubprocessGetParentUID(t *testing.T) {
	stdout, stderr, err := runSubprocess(t, nil)
	if err != nil {
		t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr output: %s", stderr)
	}
	gotStr := strings.TrimSpace(stdout)
	gotUID, err := strconv.ParseUint(gotStr, 10, 32)
	if err != nil {
		t.Fatalf("failed to parse UID %q: %v", gotStr, err)
	}
	wantUID := uint64(os.Getuid())
	if gotUID != wantUID {
		t.Errorf("got parent UID %d, want %d", gotUID, wantUID)
	}
}

func TestSubprocessCheckFlag(t *testing.T) {
	selfUID := os.Getuid()

	t.Run("matching UID", func(t *testing.T) {
		stdout, stderr, err := runSubprocess(t, nil, "-check-uid", strconv.Itoa(selfUID))
		if err != nil {
			t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
		}
		if stderr != "" {
			t.Errorf("unexpected stderr output: %s", stderr)
		}
		gotUID, err := strconv.ParseUint(strings.TrimSpace(stdout), 10, 32)
		if err != nil {
			t.Fatalf("failed to parse UID: %v", err)
		}
		if gotUID != uint64(selfUID) {
			t.Errorf("got UID %d, want %d", gotUID, selfUID)
		}
	})

	t.Run("mismatched UID", func(t *testing.T) {
		mismatchedUID := selfUID + 999
		_, stderr, err := runSubprocess(t, nil, "-check-uid", strconv.Itoa(mismatchedUID))
		if err == nil {
			t.Fatal("expected subprocess to fail with mismatched UID, but it succeeded")
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected *exec.ExitError, got: %T: %v", err, err)
		}
		if exitErr.ExitCode() != 2 {
			t.Errorf("got exit code %d, want 2", exitErr.ExitCode())
		}
		if !strings.Contains(stderr, "UID mismatch") {
			t.Errorf("expected stderr to contain 'UID mismatch', got %q", stderr)
		}
	})
}

func TestDirectGetParentUID(t *testing.T) {
	uid, err := machutils.GetParentUID()
	if err != nil {
		t.Fatalf("GetParentUID failed: %v", err)
	}
	// The parent of the test process is go test, which is owned by the
	// current user.
	if got, want := uid, uint32(os.Getuid()); got != want {
		t.Errorf("got parent UID %d, want %d", got, want)
	}
}

// TestSubprocessGetParentUIDOrphaned documents what GetParentUID reports when
// the process that launched the caller has exited. The kernel reparents an
// orphan to launchd, which runs as root, so the UID reported is root's rather
// than that of whoever actually started the process. Anything relying on
// GetParentUID as an identity has to account for this.
func TestSubprocessGetParentUIDOrphaned(t *testing.T) {
	out := filepath.Join(t.TempDir(), "parent-uid")

	// The subprocess starts a copy of itself and exits immediately, orphaning
	// the copy, which writes its report once it has been reparented.
	if _, stderr, err := runSubprocess(t, nil, "-orphan", out); err != nil {
		t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
	}

	var data []byte
	deadline := time.Now().Add(30 * time.Second)
	for {
		var err error
		if data, err = os.ReadFile(out); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the orphaned process did not report within the deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}

	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) != 2 {
		t.Fatalf("got %q, want a parent PID and UID", strings.TrimSpace(string(data)))
	}
	if got, want := fields[0], "1"; got != want {
		t.Errorf("got parent PID %v, want %v, ie. reparented to launchd", got, want)
	}
	if fields[1] == "error" {
		t.Fatalf("GetParentUID failed for the reparented process: %q", strings.TrimSpace(string(data)))
	}
	gotUID, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		t.Fatalf("failed to parse UID %q: %v", fields[1], err)
	}
	// launchd runs as root, so the reported UID is 0 and not that of the user
	// who ran the tests.
	if gotUID != 0 {
		t.Errorf("got parent UID %d, want 0, the UID of launchd", gotUID)
	}
	if gotUID == uint64(os.Getuid()) && os.Getuid() != 0 {
		t.Errorf("got the launching user's UID %d, want launchd's", gotUID)
	}
}

// executableOwner returns the UID and permissions of path, read independently
// of the package under test.
func executableOwner(t *testing.T, path string) (uint32, fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %v: %v", path, err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("stat %v: got %T, want *syscall.Stat_t", path, info.Sys())
	}
	return stat.Uid, info.Mode().Perm()
}

// cloneTestBinary copies the test binary so that it can be modified or removed
// without disturbing the tests that share it.
func cloneTestBinary(t *testing.T, perm fs.FileMode) string {
	t.Helper()
	data, err := os.ReadFile(testBinary)
	if err != nil {
		t.Fatalf("read %v: %v", testBinary, err)
	}
	clone := filepath.Join(t.TempDir(), "machutils-test-clone")
	if err := os.WriteFile(clone, data, perm); err != nil { //nolint:gosec // G703: clone is in t.TempDir()
		t.Fatalf("write %v: %v", clone, err)
	}
	// WriteFile is subject to the umask, so set the mode explicitly.
	if err := os.Chmod(clone, perm); err != nil {
		t.Fatalf("chmod %v: %v", clone, err)
	}
	return clone
}

func runBinary(t *testing.T, bin string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestDirectGetExecutableInfo(t *testing.T) {
	uid, fi, err := machutils.GetExecutableInfo()
	if err != nil {
		t.Fatalf("GetExecutableInfo failed: %v", err)
	}
	// The test binary was built by, and so is owned by, whoever is running
	// the tests.
	if got, want := uid, uint32(os.Getuid()); got != want {
		t.Errorf("got UID %d, want %d", got, want)
	}
	if fi == nil {
		t.Fatal("got nil FileInfo")
	}
	if fi.IsDir() {
		t.Errorf("expected executable file, got directory")
	}
	perm := fi.Mode().Perm()
	// It is running, so it must at least be executable by its owner, and the
	// permissions must be permission bits only.
	if perm&0100 == 0 {
		t.Errorf("got permissions %v, want the owner execute bit set", perm)
	}
	if perm&^fs.FileMode(0777) != 0 {
		t.Errorf("got %v, want permission bits only", perm)
	}
	// It reports properties of the executable file rather than anything that
	// varies between calls.
	againUID, againFI, err := machutils.GetExecutableInfo()
	if err != nil {
		t.Fatalf("GetExecutableInfo failed on the second call: %v", err)
	}
	if againUID != uid || againFI.Mode() != fi.Mode() {
		t.Errorf("got %d/%v on the second call, want %d/%v", againUID, againFI.Mode(), uid, fi.Mode())
	}
}

// TestSubprocessGetExecutableInfo verifies that a process reports the
// owner and permissions of its own executable: the subprocess discovers its
// path for itself, while the test stats the binary it launched, so the two are
// arrived at independently.
func TestSubprocessGetExecutableInfo(t *testing.T) {
	stdout, stderr, err := runSubprocess(t, nil, "-exec-uid")
	if err != nil {
		t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr output: %s", stderr)
	}
	fields := strings.Fields(strings.TrimSpace(stdout))
	if len(fields) != 2 {
		t.Fatalf("got %q, want a UID and permissions", strings.TrimSpace(stdout))
	}
	gotUID, err := strconv.ParseUint(fields[0], 10, 32)
	if err != nil {
		t.Fatalf("failed to parse UID %q: %v", fields[0], err)
	}
	gotPerm, err := strconv.ParseUint(fields[1], 8, 32)
	if err != nil {
		t.Fatalf("failed to parse permissions %q: %v", fields[1], err)
	}

	wantUID, wantPerm := executableOwner(t, testBinary)
	if uint32(gotUID) != wantUID {
		t.Errorf("got UID %d, want %d, the owner of %v", gotUID, wantUID, testBinary)
	}
	if fs.FileMode(gotPerm) != wantPerm {
		t.Errorf("got permissions %v, want %v", fs.FileMode(gotPerm), wantPerm)
	}
}

// TestSubprocessGetExecutableInfoDeleted verifies that an executable which
// no longer exists is reported as an error rather than as a UID of zero, which
// would read as root.
func TestSubprocessGetExecutableInfoDeleted(t *testing.T) {
	clone := cloneTestBinary(t, 0700)
	stdout, stderr, err := runBinary(t, clone, "-exec-uid", "-delete-self")
	if err == nil {
		t.Fatalf("expected the subprocess to fail, it printed %q", stdout)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	// Exit code 3 is reserved by the test binary for a failure to read the
	// executable's owner, distinguishing it from the other failure modes.
	if got, want := exitErr.ExitCode(), 3; got != want {
		t.Errorf("got exit code %d, want %d (stderr: %s)", got, want, stderr)
	}
	if !strings.Contains(stderr, "no such file") {
		t.Errorf("got stderr %q, want it to report the missing executable", stderr)
	}
	if got := strings.TrimSpace(stdout); got != "" {
		t.Errorf("got stdout %q, want nothing to be printed on failure", got)
	}
}

// TestEnsureParentProcessSafe verifies the check that an executable can only
// be launched by its owner. The test process is the parent of the subprocess
// and both are owned by the current user, so the outcome turns on the
// executable's permissions.
func TestEnsureParentProcessSafe(t *testing.T) {
	for _, tc := range []struct {
		name string
		perm fs.FileMode
		want string // empty when the process is expected to be safe
	}{
		{"owner only", 0700, ""},
		{"owner read and execute", 0500, ""},
		{"group executable", 0710, "group or world executable"},
		{"world executable", 0701, "group or world executable"},
		{"world readable and executable", 0755, "group or world executable"},
		{"group writable", 0720, "group or world writable"},
		{"world writable", 0702, "group or world writable"},
		{"group and world writable", 0777, "group or world writable"},
		{"setuid", 0700 | fs.ModeSetuid, "setuid or setgid"},
		{"setgid", 0700 | fs.ModeSetgid, "setuid or setgid"},
	} {
		clone := cloneTestBinary(t, tc.perm)
		stdout, stderr, err := runBinary(t, clone, "-ensure-safe")
		if tc.want == "" {
			if err != nil {
				t.Errorf("%v: expected the process to be safe: %v (stderr: %s)", tc.name, err, stderr)
				continue
			}
			if got := strings.TrimSpace(stdout); got != "safe" {
				t.Errorf("%v: got %q, want %q", tc.name, got, "safe")
			}
			continue
		}
		if err == nil {
			t.Errorf("%v: expected the process to be rejected, it printed %q", tc.name, stdout)
			continue
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Errorf("%v: expected *exec.ExitError, got %T: %v", tc.name, err, err)
			continue
		}
		// Exit code 4 is reserved by the test binary for an unsafe parent.
		if got, want := exitErr.ExitCode(), 4; got != want {
			t.Errorf("%v: got exit code %d, want %d (stderr: %s)", tc.name, got, want, stderr)
		}
		if !strings.Contains(stderr, tc.want) {
			t.Errorf("%v: got stderr %q, want it to contain %q", tc.name, stderr, tc.want)
		}
	}
}

// TestEnsureParentProcessSafeOrphaned verifies that an orphaned process
// (reparented to launchd) is rejected by EnsureParentProcessSafe.
func TestEnsureParentProcessSafeOrphaned(t *testing.T) {
	out := filepath.Join(t.TempDir(), "orphan-safe-result")

	if _, stderr, err := runSubprocess(t, nil, "-orphan-safe", out); err != nil {
		t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
	}

	var data []byte
	deadline := time.Now().Add(30 * time.Second)
	for {
		var err error
		if data, err = os.ReadFile(out); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the orphaned process did not report within the deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}

	result := strings.TrimSpace(string(data))
	if !strings.Contains(result, "process has been orphaned") {
		t.Errorf("got %q, want it to report that the process has been orphaned", result)
	}
}

// TestSubprocessGetExecutableInfoViaSymlink verifies that the permissions and
// owner reported are those of the binary rather than of a symbolic link
// leading to it. os.Executable reports the path the process was executed
// through, which for a link is the link itself, and a link's own mode is
// 0755: were it not resolved, every such process would look group and world
// executable and be rejected as unsafe.
func TestSubprocessGetExecutableInfoViaSymlink(t *testing.T) {
	clone := cloneTestBinary(t, 0700)
	link := filepath.Join(filepath.Dir(clone), "machutils-test-link")
	if err := os.Symlink(clone, link); err != nil {
		t.Fatal(err)
	}
	// The link is not the binary: its own mode differs from the mode that
	// must be reported.
	li, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if li.Mode().Perm() == 0700 {
		t.Fatalf("the symlink has the same permissions as its target, so this proves nothing")
	}

	stdout, stderr, err := runBinary(t, link, "-exec-uid")
	if err != nil {
		t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
	}
	want := fmt.Sprintf("%d %o\n", os.Getuid(), 0700)
	if got := stdout; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Running through the link must also be judged safe, which it is only
	// because the link's own permissions are not the ones examined.
	stdout, stderr, err = runBinary(t, link, "-ensure-safe")
	if err != nil {
		t.Fatalf("running via a symlink was judged unsafe: %v (stderr: %s)", err, stderr)
	}
	if got, want := strings.TrimSpace(stdout), "safe"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestSubprocessGetExecutableInfoUnresolvable verifies that an executable
// whose path cannot be resolved is reported with the path named, so that the
// failure says which binary could not be examined.
func TestSubprocessGetExecutableInfoUnresolvable(t *testing.T) {
	clone := cloneTestBinary(t, 0700)
	stdout, stderr, err := runBinary(t, clone, "-exec-uid", "-delete-self")
	if err == nil {
		t.Fatalf("expected the subprocess to fail, it printed %q", stdout)
	}
	if got, want := stderr, clone; !strings.Contains(got, want) {
		t.Errorf("the error %q does not name the executable %q", got, want)
	}
}

// TestSubprocessExecutablePath verifies that the kernel is the source of the
// executable's path. Launching through a symbolic link is the case that can be
// reproduced here: os.Executable reports the link, because that is the path
// the process was launched with, whilst the kernel reports the binary. The
// same distinction is what makes the path usable from an App Sandbox, where
// os.Executable can name a path that does not exist.
func TestSubprocessExecutablePath(t *testing.T) {
	clone := cloneTestBinary(t, 0700)
	link := filepath.Join(filepath.Dir(clone), "machutils-path-link")
	if err := os.Symlink(clone, link); err != nil {
		t.Fatal(err)
	}
	// The kernel reports the canonical path, which on macOS differs from the
	// one under test even before the symlink to the binary is considered:
	// temporary directories live under /var, itself a link to /private/var.
	resolved, err := filepath.EvalSymlinks(clone)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == clone {
		t.Logf("the temporary directory is already canonical: %v", clone)
	}

	for _, tc := range []struct {
		name            string
		run             string
		wantLaunchedAs  string
		wantExecutingAs string
	}{
		{"launched directly", clone, clone, resolved},
		{"launched through a symlink", link, link, resolved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runBinary(t, tc.run, "-exec-paths")
			if err != nil {
				t.Fatalf("subprocess failed: %v (stderr: %s)", err, stderr)
			}
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			if len(lines) != 2 {
				t.Fatalf("expected two paths, got %q", stdout)
			}
			if got, want := lines[0], tc.wantLaunchedAs; got != want {
				t.Errorf("os.Executable: got %v, want %v", got, want)
			}
			if got, want := lines[1], tc.wantExecutingAs; got != want {
				t.Errorf("ExecutablePath: got %v, want %v", got, want)
			}
		})
	}
}

// TestExecutablePath verifies the path reported for the test binary itself,
// which is launched directly and so is reported the same way by both.
func TestExecutablePath(t *testing.T) {
	got, err := machutils.ExecutablePath()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// os.Executable is not resolved, so compare against the resolved form.
	if want, err = filepath.EvalSymlinks(want); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
