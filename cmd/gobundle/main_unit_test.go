// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"cloudeng.io/macos/cmd/gobundle/gobundleconfig"
)

// parseFlagsFor runs parseFlags over the given command line, restoring os.Args
// afterwards. parseFlags reads os.Args itself and builds its own flag sets, so
// that is the only way to drive it.
func parseFlagsFor(t *testing.T, args ...string) (string, []string, *flag.FlagSet, *localFlags) {
	t.Helper()
	orig := os.Args
	t.Cleanup(func() { os.Args = orig })
	os.Args = args
	execBin, execArgs, cmdFs, lf, err := parseFlags()
	if err != nil {
		t.Fatalf("parseFlags failed: %v", err)
	}
	if cmdFs == nil || lf == nil {
		t.Fatalf("parseFlags returned cmdFs=%v lf=%v, want both", cmdFs, lf)
	}
	return execBin, execArgs, cmdFs, lf
}

func TestParseFlagsNormal(t *testing.T) {
	execBin, execArgs, cmdFs, lf := parseFlagsFor(t,
		"gobundle", "--verbose", "--shared-config", "shared.yaml", "build", ".")

	if execBin != "" {
		t.Errorf("expected empty execBinary, got %q", execBin)
	}
	if len(execArgs) != 0 {
		t.Errorf("expected empty execArgs, got %v", execArgs)
	}
	if !lf.Verbose {
		t.Error("expected Verbose to be true")
	}
	if lf.SharedConfig != "shared.yaml" {
		t.Errorf("SharedConfig = %q, want shared.yaml", lf.SharedConfig)
	}
	if lf.Notarize {
		t.Error("expected Notarize to be false, it was not given")
	}
	// What is left over is the go command line, which main passes to
	// parseGoArgs.
	if got, want := cmdFs.Args(), []string{"build", "."}; !slices.Equal(got, want) {
		t.Errorf("cmdFs.Args = %v, want %v", got, want)
	}
}

func TestParseFlagsSignThenRun(t *testing.T) {
	execBin, execArgs, cmdFs, lf := parseFlagsFor(t,
		"gobundle", "/path/to/binary", "--verbose", "--notarize", signThenRunVerb, "arg1", "arg2")

	if execBin != "/path/to/binary" {
		t.Errorf("execBin = %q, want /path/to/binary", execBin)
	}
	if got, want := execArgs, []string{"arg1", "arg2"}; !slices.Equal(got, want) {
		t.Errorf("execArgs = %v, want %v", got, want)
	}
	if !lf.Verbose {
		t.Error("expected Verbose to be true")
	}
	if !lf.Notarize {
		t.Error("expected Notarize to be true")
	}
	// The flag set returned is the one parsed from after the binary, so what
	// remains starts at the verb.
	if got, want := cmdFs.Args(), []string{signThenRunVerb, "arg1", "arg2"}; !slices.Equal(got, want) {
		t.Errorf("cmdFs.Args = %v, want %v", got, want)
	}
}

// TestParseFlagsSignThenRunUsesAFreshFlagSet verifies that the flags reported
// for the sign-then-run form come only from the second parse. parseFlags parses
// twice, and a flag.FlagSet carries values and the record of which flags were
// set from one Parse to the next, so reusing the first set would report
// --verbose here even though it appears before the binary and is therefore not
// part of the command line that form describes.
func TestParseFlagsSignThenRunUsesAFreshFlagSet(t *testing.T) {
	_, _, _, lf := parseFlagsFor(t,
		"gobundle", "--verbose", "/path/to/binary", signThenRunVerb, "arg1")

	if lf.Verbose {
		t.Error("Verbose was carried over from the first parse")
	}
}

// captureStderr collects what fn writes to os.Stderr. flag writes its
// diagnostics there, and resolves the destination when it writes, so
// redirecting for the duration of the call is enough.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	collected := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		collected <- buf.String()
	}()
	fn()
	os.Stderr = orig
	w.Close()
	return <-collected
}

// TestParseFlagsErrors covers a flag that cannot be parsed. createFS builds its
// flag sets with flag.ContinueOnError, so this is reported to the caller rather
// than ending the process; both parses are covered, the second of which runs
// only for the sign-then-run form.
func TestParseFlagsErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"in the first parse", []string{"gobundle", "--unknown-flag"}},
		{"in the second parse", []string{"gobundle", "/path/to/bin", "--unknown-flag", signThenRunVerb}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orig := os.Args
			t.Cleanup(func() { os.Args = orig })
			os.Args = tc.args

			var (
				execBin  string
				execArgs []string
				cmdFs    *flag.FlagSet
				lf       *localFlags
				err      error
			)
			stderr := captureStderr(t, func() {
				execBin, execArgs, cmdFs, lf, err = parseFlags()
			})

			if err == nil {
				t.Fatal("expected an error")
			}
			if got, want := err.Error(), "unknown-flag"; !strings.Contains(got, want) {
				t.Errorf("error %q does not name %q", got, want)
			}
			// Nothing usable accompanies the error: main tests err first and
			// must not be handed a half parsed flag set.
			if execBin != "" || execArgs != nil || cmdFs != nil || lf != nil {
				t.Errorf("got execBin=%q execArgs=%v cmdFs=%v lf=%v, want all zero",
					execBin, execArgs, cmdFs, lf)
			}
			// The flag package reports what was wrong, and follows it with
			// the usage message.
			for _, want := range []string{"not defined", "Usage of gobundle"} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr %q does not contain %q", stderr, want)
				}
			}
		})
	}
}

func TestParseGoArgs_Empty(t *testing.T) {
	bin, verb, args := parseGoArgs(nil)
	if bin != "" || verb != "" || args != nil {
		t.Errorf("expected empty results for nil args, got bin=%q, verb=%q, args=%v", bin, verb, args)
	}
}

func TestParseGoArgs_Build(t *testing.T) {
	bin, verb, args := parseGoArgs([]string{"build", "-o", "custom_bin", "main.go"})
	if verb != "build" {
		t.Errorf("verb = %q, want build", verb)
	}
	if bin != "custom_bin" {
		t.Errorf("bin = %q, want custom_bin", bin)
	}
	if len(args) != 3 || args[0] != "-o" {
		t.Errorf("args = %v, want [-o custom_bin main.go]", args)
	}
}

func TestParseGoArgs_Install(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GOBIN", tmpDir)
	bin, verb, args := parseGoArgs([]string{"install", "main.go"})
	if verb != "install" {
		t.Errorf("verb = %q, want install", verb)
	}
	if bin != filepath.Join(tmpDir, "main") {
		t.Errorf("bin = %q, want %s", bin, filepath.Join(tmpDir, "main"))
	}
	if len(args) != 1 || args[0] != "main.go" {
		t.Errorf("args = %v, want [main.go]", args)
	}
}

func TestParseGoArgs_RunAndOther(t *testing.T) {
	bin, verb, args := parseGoArgs([]string{"run", "main.go", "arg1"})
	if verb != "run" {
		t.Errorf("verb = %q, want run", verb)
	}
	if bin != "" {
		t.Errorf("bin = %q, want empty", bin)
	}
	if len(args) != 2 || args[0] != "main.go" || args[1] != "arg1" {
		t.Errorf("args = %v, want [main.go arg1]", args)
	}

	bin, verb, args = parseGoArgs([]string{"version"})
	if verb != "version" || bin != "" || len(args) != 0 {
		t.Errorf("unexpected parseGoArgs version: bin=%q, verb=%q, args=%v", bin, verb, args)
	}
}

func TestIsDir(t *testing.T) {
	tmpDir := t.TempDir()
	if !isDir(tmpDir) {
		t.Errorf("isDir(%q) = false, want true", tmpDir)
	}
	nonExistent := filepath.Join(tmpDir, "nonexistent")
	if isDir(nonExistent) {
		t.Errorf("isDir(%q) = true, want false", nonExistent)
	}
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if isDir(filePath) {
		t.Errorf("isDir(%q) = true, want false", filePath)
	}
}

func TestWithDir(t *testing.T) {
	if got := withDir("", "/path/to/mybin"); got != "mybin" {
		t.Errorf("withDir(\"\", ...) = %q, want mybin", got)
	}
	if got := withDir("/out/dir", "/path/to/mybin"); got != filepath.Join("/out/dir", "mybin") {
		t.Errorf("withDir(/out/dir, ...) = %q, want %s", got, filepath.Join("/out/dir", "mybin"))
	}
}

func TestConsumeBuildArgs(t *testing.T) {
	dashO, rest := consumeBuildArgs([]string{"-v", "-o", "/out/bin", "-tags", "foo", "main.go"})
	if dashO != "/out/bin" {
		t.Errorf("dashO = %q, want /out/bin", dashO)
	}
	if len(rest) != 1 || rest[0] != "main.go" {
		t.Errorf("rest = %v, want [main.go]", rest)
	}
}

func TestDetermineBuildBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. -o pointing to file
	bin := determineBuildBinary("/out/file", []string{"main.go"})
	if bin != "/out/file" {
		t.Errorf("bin = %q, want /out/file", bin)
	}

	// 2. -o pointing to directory
	bin = determineBuildBinary(tmpDir, []string{"main.go"})
	if bin != filepath.Join(tmpDir, "main") {
		t.Errorf("bin = %q, want %s", bin, filepath.Join(tmpDir, "main"))
	}

	// 3. no -o, rest with .go
	bin = determineBuildBinary("", []string{"custom.go"})
	if bin != "custom" {
		t.Errorf("bin = %q, want custom", bin)
	}

	// 4. no -o, rest empty or "."
	bin = determineBuildBinary("", nil)
	cwd, _ := os.Getwd()
	if bin != filepath.Base(cwd) {
		t.Errorf("bin = %q, want %s", bin, filepath.Base(cwd))
	}

	bin = determineBuildBinary("", []string{"."})
	if bin != filepath.Base(cwd) {
		t.Errorf("bin = %q, want %s", bin, filepath.Base(cwd))
	}

	// getBuildBinaryAbs
	absBin := getBuildBinaryAbs([]string{"-o", "/out/bin", "main.go"})
	if absBin != "/out/bin" {
		t.Errorf("getBuildBinaryAbs = %q, want /out/bin", absBin)
	}
}

func TestGetInstallBinaryAbs(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatal(err)
	}

	// 1. GOBIN
	t.Setenv("GOBIN", binDir)
	t.Setenv("GOPATH", "")
	t.Setenv("HOME", "")
	if got, want := getInstallBinaryAbs([]string{"pkg/cmd/app"}), filepath.Join(binDir, "app"); got != want {
		t.Errorf("getInstallBinaryAbs with GOBIN = %q, want %q", got, want)
	}

	// 2. GOPATH
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", tmpDir)
	if got, want := getInstallBinaryAbs([]string{"pkg/cmd/app"}), filepath.Join(binDir, "app"); got != want {
		t.Errorf("getInstallBinaryAbs with GOPATH = %q, want %q", got, want)
	}

	// 3. HOME
	homeDir := filepath.Join(tmpDir, "home")
	homeBin := filepath.Join(homeDir, "go", "bin")
	if err := os.MkdirAll(homeBin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	t.Setenv("HOME", homeDir)
	if got, want := getInstallBinaryAbs([]string{"pkg/cmd/app"}), filepath.Join(homeBin, "app"); got != want {
		t.Errorf("getInstallBinaryAbs with HOME = %q, want %q", got, want)
	}

	// 4. Fallback (no env)
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	t.Setenv("HOME", "")
	if got, want := getInstallBinaryAbs([]string{"pkg/cmd/app"}), "app"; got != want {
		t.Errorf("getInstallBinaryAbs fallback = %q, want %q", got, want)
	}
}

func TestRunCommand(t *testing.T) {
	ctx := context.Background()

	// Successful command
	if err := runCommand(ctx, "echo", []string{"hello"}); err != nil {
		t.Errorf("runCommand(echo) failed: %v", err)
	}

	// Failing command
	if err := runCommand(ctx, "nonexistent-binary-that-should-fail", nil); err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestHandleIcons(t *testing.T) {
	// 1. Empty icon returns no-op cleanup
	b := newBundle(gobundleconfig.T{})
	cleanup, err := b.handleIcons()
	if err != nil {
		t.Fatalf("handleIcons for empty icon failed: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected non-nil cleanup function")
	}
	cleanup()

	// 2. With icon specified
	tmpDir := t.TempDir()
	iconFile := filepath.Join(tmpDir, "app.png")
	if err := os.WriteFile(iconFile, []byte("fake-png"), 0600); err != nil {
		t.Fatal(err)
	}
	bWithIcon := newBundle(gobundleconfig.T{Icon: iconFile})
	cleanupWithIcon, err := bWithIcon.handleIcons()
	if err != nil {
		t.Fatalf("handleIcons with icon failed: %v", err)
	}
	cleanupWithIcon()
}

func TestPrintf(t *testing.T) {
	verboseLog.Reset()
	verbose = false
	printf("message %d\n", 1)
	if got, want := verboseLog.String(), "message 1\n"; got != want {
		t.Errorf("verboseLog = %q, want %q", got, want)
	}

	verboseLog.Reset()
	verbose = true
	defer func() { verbose = false }()
	printf("message %d\n", 2)
	if got, want := verboseLog.String(), "message 2\n"; got != want {
		t.Errorf("verboseLog = %q, want %q", got, want)
	}
}

func TestRunAndExit(t *testing.T) {
	_ = t
	// Test success path: fn returns nil
	// runAndExit calls os.Exit(0), so we test in a subprocess
	if os.Getenv("TEST_RUN_AND_EXIT") == "1" {
		runAndExit(func() error {
			return nil
		})
		return
	}
}

func TestHandleGoBuildDirect(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	bin := filepath.Join(tmpDir, "example")
	cfg := gobundleconfig.T{Path: filepath.Join(tmpDir, "example.app")}.WithDefaultsForBinary(bin)

	err := handleGoBuild(ctx, cfg, false, bin, []string{"-o", bin, "./testdata/example.go"})
	if err != nil {
		t.Fatalf("handleGoBuild failed: %v", err)
	}

	// Verify the symlink and bundle were created
	info, err := os.Lstat(bin)
	if err != nil {
		t.Fatalf("failed to stat bin: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink", bin)
	}

	// Failure case: invalid build argument
	err = handleGoBuild(ctx, cfg, false, bin, []string{"--invalid-flag"})
	if err == nil {
		t.Error("expected error for invalid build flags")
	}
}

func TestHandleGoInstallDirect(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Setenv("GOBIN", tmpDir)
	bin := filepath.Join(tmpDir, "example")
	cfg := gobundleconfig.T{}.WithDefaultsForBinary(bin)

	err := handleGoInstall(ctx, cfg, false, bin, []string{"./testdata/example.go"})
	if err != nil {
		t.Fatalf("handleGoInstall failed: %v", err)
	}

	// Verify symlink
	info, err := os.Lstat(bin)
	if err != nil {
		t.Fatalf("failed to stat installed bin: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink", bin)
	}

	// Failure case: cannot determine install dir
	err = handleGoInstall(ctx, cfg, false, "", []string{"./testdata/example.go"})
	if err == nil {
		t.Error("expected error when install directory is empty")
	}
}

func TestHandleGoRunExecDirect(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	bin := filepath.Join(tmpDir, "example")

	// Compile the example first
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/example.go")
	if err := cmd.Run(); err != nil {
		t.Fatalf("compiling example failed: %v", err)
	}

	cfg := gobundleconfig.T{}.WithDefaultsForBinary(bin)
	err := handleGoRunExec(ctx, cfg, false, bin, []string{"testarg"})
	if err != nil {
		t.Fatalf("handleGoRunExec failed: %v", err)
	}
}

func runSubprocessCase(c string) {
	switch c {
	case "exit_success":
		runAndExit(func() error { return nil })
	case "exit_error":
		runAndExit(func() error { return fmt.Errorf("sample error") })
	case "exit_direct":
		exit(42, "direct exit %d\n", 42)
	case "print_help":
		printHelpAndExit()
	case "handle_go_run_exec_flag":
		handleGoRun(context.Background(), gobundleconfig.T{}, false, []string{"-exec", "something"})
	case "main_show_config":
		os.Args = []string{"gobundle", "--show-config"}
		main()
	case "main_help":
		os.Args = []string{"gobundle", "--help"}
		main()
	case "main_noargs":
		os.Args = []string{"gobundle"}
		main()
	case "main_badflag":
		os.Args = []string{"gobundle", "--unknown-flag"}
		main()
	}
}

func TestSubprocessHelpers(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		runSubprocessCase(os.Getenv("TEST_SUBPROCESS_CASE"))
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		subCase  string
		wantCode int
	}{
		{"exit_success", "exit_success", 0},
		{"exit_error", "exit_error", 1},
		{"exit_direct", "exit_direct", 42},
		{"print_help", "print_help", 1},
		{"handle_go_run_exec_flag", "handle_go_run_exec_flag", 1},
		{"main_show_config", "main_show_config", 0},
		{"main_help", "main_help", 1},
		{"main_noargs", "main_noargs", 2},
		// A flag that cannot be parsed reaches main as an error, which it
		// treats as a request for help; printHelpAndExit exits 1.
		{"main_badflag", "main_badflag", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(exe, "-test.run=TestSubprocessHelpers$")
			cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1", "TEST_SUBPROCESS_CASE="+tc.subCase)
			err := cmd.Run()
			if tc.wantCode == 0 {
				if err != nil {
					t.Errorf("%s failed: %v", tc.name, err)
				}
				return
			}
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("%s expected ExitError, got %v", tc.name, err)
			}
			if exitErr.ExitCode() != tc.wantCode {
				t.Errorf("%s exit code = %d, want %d", tc.name, exitErr.ExitCode(), tc.wantCode)
			}
		})
	}
}
