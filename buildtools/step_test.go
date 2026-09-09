// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestStepUtilitiesBasic(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewCommandRunner()

	// 1. NoopStep
	noop := buildtools.NoopStep("test reason")
	res, err := noop.Run(ctx, runner)
	if err != nil {
		t.Fatalf("NoopStep failed: %v", err)
	}
	if !strings.Contains(res.Executable(), "noop: test reason") {
		t.Errorf("NoopStep executable = %q", res.Executable())
	}

	// 2. ErrorStep
	testErr := errors.New("custom error")
	errStep := buildtools.ErrorStep(testErr, "mycmd", "arg1", "arg2")
	res, err = errStep.Run(ctx, runner)
	if !errors.Is(err, testErr) {
		t.Errorf("ErrorStep error = %v, want %v", err, testErr)
	}
	if res.Executable() != "mycmd" || len(res.Args()) != 2 {
		t.Errorf("ErrorStep result mismatch: %+v", res)
	}

	// 3. StepResult methods
	sr := buildtools.NewStepResult("echo", []string{"hello", "world with spaces"}, []byte("line1\nline2\n"), nil)
	if sr.Executable() != "echo" {
		t.Errorf("Executable = %q", sr.Executable())
	}
	if len(sr.Args()) != 2 {
		t.Errorf("Args = %v", sr.Args())
	}
	if sr.Output() != "line1\nline2\n" {
		t.Errorf("Output = %q", sr.Output())
	}
	cmdLine := sr.CommandLine()
	if !strings.Contains(cmdLine, `"world with spaces"`) {
		t.Errorf("CommandLine should quote arguments with spaces: %q", cmdLine)
	}
	str := sr.String()
	if !strings.Contains(str, "line1") || !strings.Contains(str, "line2") {
		t.Errorf("String() = %q", str)
	}

	// 4. CommandRunner with Timing
	timingRunner := buildtools.NewCommandRunner(buildtools.WithCommandTiming(true))
	res, err = timingRunner.Run(ctx, "echo", "timing")
	if err != nil {
		t.Fatalf("timingRunner.Run failed: %v", err)
	}
	if res.Duration() < 0 {
		t.Errorf("Duration = %v", res.Duration())
	}
}

func TestStepUtilitiesWriteFileAndCWD(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewCommandRunner()

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "written.txt")
	msg, err := runner.WriteFile(ctx, outPath, []byte("data"), 0644)
	if err != nil {
		t.Fatalf("runner.WriteFile failed: %v", err)
	}
	if !strings.Contains(msg, "wrote 4 bytes") {
		t.Errorf("unexpected message: %q", msg)
	}
	content, err := os.ReadFile(outPath)
	if err != nil || string(content) != "data" {
		t.Errorf("file content mismatch: %v, %s", err, string(content))
	}

	// Dry-run WriteFile
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	dryMsg, err := dryRunner.WriteFile(ctx, filepath.Join(tempDir, "dry.txt"), []byte("data"), 0644)
	if err != nil {
		t.Fatalf("dry runner WriteFile failed: %v", err)
	}
	if !strings.Contains(dryMsg, "write 4 bytes") {
		t.Errorf("unexpected dry message: %q", dryMsg)
	}

	// ContextWithCWD and CWDFromContext
	customCWD := "/tmp/custom_cwd"
	ctxWithCWD := buildtools.ContextWithCWD(ctx, customCWD)
	if got := buildtools.CWDFromContext(ctxWithCWD); got != customCWD {
		t.Errorf("CWDFromContext = %q, want %q", got, customCWD)
	}
	// Context without CWD returns process CWD
	if got := buildtools.CWDFromContext(ctx); got == "" {
		t.Error("CWDFromContext on default ctx should return non-empty process CWD")
	}
}
