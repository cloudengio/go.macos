// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestSwiftApp(t *testing.T) {
	if _, err := exec.LookPath("swift"); err != nil {
		t.Skip("swift is not available in PATH")
	}

	tmpDir := t.TempDir()
	// Initialize a minimal swift package
	cmd := exec.Command("swift", "package", "init", "--type", "executable")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to initialize swift package: %v, output: %s", err, string(out))
	}

	ctx := context.Background()

	// 1. Debug configuration
	debugApp := buildtools.NewSwiftApp(ctx, tmpDir, false)
	debugBin := debugApp.BinDir()
	if !strings.Contains(debugBin, "debug") {
		t.Errorf("expected debug in bin dir, got: %s", debugBin)
	}

	// 2. Release configuration
	releaseApp := buildtools.NewSwiftApp(ctx, tmpDir, true)
	releaseBin := releaseApp.BinDir()
	if !strings.Contains(releaseBin, "release") {
		t.Errorf("expected release in bin dir, got: %s", releaseBin)
	}

	// 3. ExecutablePath
	exePath := debugApp.ExecutablePath("mytool")
	if exePath != filepath.Join(debugBin, "mytool") {
		t.Errorf("expected %s, got %s", filepath.Join(debugBin, "mytool"), exePath)
	}

	// 4. Build step in dry-run mode
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	step := debugApp.Build()
	res, err := step.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedArgs := []string{"build"}
	if strings.Join(res.Args(), " ") != strings.Join(expectedArgs, " ") {
		t.Errorf("expected %v, got %v", expectedArgs, res.Args())
	}

	relStep := releaseApp.Build()
	res, err = relStep.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedRelArgs := []string{"build", "--configuration", "release"}
	if strings.Join(res.Args(), " ") != strings.Join(expectedRelArgs, " ") {
		t.Errorf("expected %v, got %v", expectedRelArgs, res.Args())
	}

	// 5. CopyIcons
	icons := []buildtools.IconSet{
		{
			Dir:  filepath.Join(tmpDir, "MyIcon.iconset"),
			Name: "CustomAppIcon.icns",
		},
		{
			Dir: filepath.Join(tmpDir, "Default.iconset"),
			// Name empty, should default to AppIcon.icns
		},
	}
	copySteps := debugApp.CopyIcons(icons)
	if len(copySteps) != 2 {
		t.Fatalf("expected 2 copy steps, got %d", len(copySteps))
	}
	res, err = copySteps[0].Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedCopy0 := []string{
		filepath.Join(tmpDir, "MyIcon.iconset", "CustomAppIcon.icns"),
		filepath.Join(tmpDir, "Resources", "CustomAppIcon.icns"),
	}
	if strings.Join(res.Args(), " ") != strings.Join(expectedCopy0, " ") {
		t.Errorf("expected %v, got %v", expectedCopy0, res.Args())
	}

	res, err = copySteps[1].Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedCopy1 := []string{
		filepath.Join(tmpDir, "Default.iconset", "AppIcon.icns"),
		filepath.Join(tmpDir, "Resources", "AppIcon.icns"),
	}
	if strings.Join(res.Args(), " ") != strings.Join(expectedCopy1, " ") {
		t.Errorf("expected %v, got %v", expectedCopy1, res.Args())
	}
}

func TestSwiftAppPanicOnInvalidDir(t *testing.T) {
	if _, err := exec.LookPath("swift"); err != nil {
		t.Skip("swift is not available in PATH")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected NewSwiftApp to panic on invalid package dir, but did not")
		}
	}()

	// A non-existent directory will cause CommandRunner to fail when changing directory.
	_ = buildtools.NewSwiftApp(context.Background(), "/nonexistent/swift/dir/path", false)
}
