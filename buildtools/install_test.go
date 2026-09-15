// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"os"
	"path/filepath"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestInstallDirExplicit(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()

	nonExistent := filepath.Join(tempDir, "does-not-exist")
	readOnlyDir := filepath.Join(tempDir, "read-only")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0755)
	})

	writableDir := filepath.Join(tempDir, "writable")
	if err := os.Mkdir(writableDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Skips non-existent and read-only, chooses first writable
	dir, err := buildtools.InstallDir(ctx, nonExistent, readOnlyDir, writableDir)
	if err != nil {
		t.Fatalf("InstallDir failed: %v", err)
	}
	if dir != writableDir {
		t.Errorf("got %q, want %q", dir, writableDir)
	}

	// 2. All invalid returns error
	if _, err := buildtools.InstallDir(ctx, nonExistent, readOnlyDir); err == nil {
		t.Error("expected error when no writable directory exists")
	}
}

func TestInstallDirEnv(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()

	gobinDir := filepath.Join(tempDir, "gobin")
	if err := os.Mkdir(gobinDir, 0755); err != nil {
		t.Fatal(err)
	}
	pathDir := filepath.Join(tempDir, "path-bin")
	if err := os.Mkdir(pathDir, 0755); err != nil {
		t.Fatal(err)
	}

	// When GOBIN is set and exists/writable, it is preferred
	t.Setenv("GOBIN", gobinDir)
	t.Setenv("PATH", pathDir)

	dir, err := buildtools.InstallDir(ctx)
	if err != nil {
		t.Fatalf("InstallDir failed: %v", err)
	}
	if dir != gobinDir {
		t.Errorf("got %q, want %q", dir, gobinDir)
	}

	// When GOBIN does not exist, falls back to PATH
	t.Setenv("GOBIN", filepath.Join(tempDir, "missing-gobin"))
	dir, err = buildtools.InstallDir(ctx)
	if err != nil {
		t.Fatalf("InstallDir fallback to PATH failed: %v", err)
	}
	if dir != pathDir {
		t.Errorf("got %q, want %q", dir, pathDir)
	}
}

func setupTestBundle(t *testing.T, tempDir string) (buildtools.AppBundle, []byte) {
	t.Helper()
	srcBundleDir := filepath.Join(tempDir, "src", "MyApp.app")
	bundle := buildtools.AppBundle{
		Path: srcBundleDir,
		Info: buildtools.InfoPlist{
			CFBundleIdentifier: "io.cloudeng.myapp",
		}.WithDefaults("myapp"),
	}
	stepRunner := buildtools.NewRunner()
	stepRunner.AddSteps(bundle.Create()...)
	stepRunner.AddSteps(bundle.WriteInfoPlist())
	results := stepRunner.Run(t.Context(), buildtools.NewCommandRunner())
	if err := results.Error(); err != nil {
		t.Fatalf("bundle creation failed: %v", err)
	}

	binContent := []byte("#!/bin/sh\necho hello\n")
	exePath := filepath.Join(bundle.Path, "Contents", "MacOS", "myapp")
	if err := os.WriteFile(exePath, binContent, 0600); err != nil {
		t.Fatal(err)
	}
	return bundle, binContent
}

func TestAppBundleInstallWithSoftlink(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	bundle, binContent := setupTestBundle(t, tempDir)

	installDir := filepath.Join(tempDir, "install-target")
	if err := os.Mkdir(installDir, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := bundle.Install(true, installDir).Run(ctx, runner); err != nil {
		t.Fatalf("Install with softlink failed: %v", err)
	}

	installedBundle := filepath.Join(installDir, "MyApp.app")
	if fi, err := os.Stat(installedBundle); err != nil || !fi.IsDir() {
		t.Fatalf("expected installed bundle at %s, err: %v", installedBundle, err)
	}

	installedLink := filepath.Join(installDir, "myapp")
	fi, err := os.Lstat(installedLink)
	if err != nil {
		t.Fatalf("expected symlink at %s, err: %v", installedLink, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink mode, got %v", fi.Mode())
	}
	dest, err := os.Readlink(installedLink)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	expectedDest := filepath.Join("MyApp.app", "Contents", "MacOS", "myapp")
	if dest != expectedDest {
		t.Errorf("symlink target = %q, want %q", dest, expectedDest)
	}

	readContent, err := os.ReadFile(installedLink)
	if err != nil {
		t.Fatalf("reading through symlink failed: %v", err)
	}
	if string(readContent) != string(binContent) {
		t.Errorf("content through symlink = %q, want %q", string(readContent), string(binContent))
	}
}

func TestAppBundleInstallWithoutSoftlink(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	bundle, _ := setupTestBundle(t, tempDir)

	installDir := filepath.Join(tempDir, "install-target")
	if err := os.Mkdir(installDir, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := bundle.Install(false, installDir).Run(ctx, runner); err != nil {
		t.Fatalf("Install without softlink failed: %v", err)
	}

	installedBundle := filepath.Join(installDir, "MyApp.app")
	if fi, err := os.Stat(installedBundle); err != nil || !fi.IsDir() {
		t.Fatalf("expected installed bundle at %s, err: %v", installedBundle, err)
	}
	if _, err := os.Lstat(filepath.Join(installDir, "myapp")); !os.IsNotExist(err) {
		t.Errorf("expected no symlink in installDir, got err: %v", err)
	}
}

func TestAppBundleInstallDryRunAndSameDir(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	bundle, _ := setupTestBundle(t, tempDir)

	// Dry-run install
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	installDir := filepath.Join(tempDir, "install-dryrun")
	if err := os.Mkdir(installDir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.Install(true, installDir).Run(ctx, dryRunner); err != nil {
		t.Fatalf("dry-run install failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "MyApp.app")); !os.IsNotExist(err) {
		t.Errorf("expected dry-run to not create files")
	}

	// Install when bundle is already in installDir
	runner := buildtools.NewCommandRunner()
	if _, err := bundle.Install(true, filepath.Dir(bundle.Path)).Run(ctx, runner); err != nil {
		t.Fatalf("installing bundle in-place failed: %v", err)
	}
}

func TestAppBundleInstallErrors(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewCommandRunner()
	tempDir := t.TempDir()

	// Empty path
	var emptyBundle buildtools.AppBundle
	if _, err := emptyBundle.Install(false, tempDir).Run(ctx, runner); err == nil {
		t.Error("expected empty bundle path to fail")
	}

	// Non-existent bundle
	missingBundle := buildtools.AppBundle{Path: filepath.Join(tempDir, "Missing.app")}
	if _, err := missingBundle.Install(false, tempDir).Run(ctx, runner); err == nil {
		t.Error("expected non-existent bundle path to fail")
	}

	// No writable install dir
	readOnlyDir := filepath.Join(tempDir, "ro")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0755) })

	validBundleDir := filepath.Join(tempDir, "Valid.app")
	if err := os.MkdirAll(filepath.Join(validBundleDir, "Contents", "MacOS"), 0755); err != nil {
		t.Fatal(err)
	}
	validBundle := buildtools.AppBundle{
		Path: validBundleDir,
		Info: buildtools.InfoPlist{CFBundleExecutable: "bin"},
	}
	if _, err := validBundle.Install(false, readOnlyDir).Run(ctx, runner); err == nil {
		t.Error("expected install to read-only dir to fail")
	}

	// Missing executable when softlink is requested
	noExeBundle := buildtools.AppBundle{
		Path: validBundleDir,
	}
	if _, err := noExeBundle.Install(true, tempDir).Run(ctx, runner); err == nil {
		t.Error("expected install with softlink and no executable to fail")
	}
}

func TestReadInfoPlist(t *testing.T) {
	tempDir := t.TempDir()
	plistPath := filepath.Join(tempDir, "Info.plist")

	info := buildtools.InfoPlist{
		CFBundleIdentifier:     "io.cloudeng.sample",
		CFBundleName:           "Sample",
		LSMinimumSystemVersion: "15.0",
	}.WithDefaults("sample-bin")

	runner := buildtools.NewCommandRunner()
	bundle := buildtools.AppBundle{
		Path: filepath.Join(tempDir, "Sample.app"),
		Info: info,
	}
	if err := os.MkdirAll(filepath.Join(bundle.Path, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.WriteInfoPlist().Run(t.Context(), runner); err != nil {
		t.Fatalf("WriteInfoPlist failed: %v", err)
	}

	readInfo, err := buildtools.ReadInfoPlist(filepath.Join(bundle.Path, "Contents", "Info.plist"))
	if err != nil {
		t.Fatalf("ReadInfoPlist failed: %v", err)
	}
	if readInfo.CFBundleExecutable != "sample-bin" {
		t.Errorf("CFBundleExecutable = %q, want sample-bin", readInfo.CFBundleExecutable)
	}
	if readInfo.CFBundleIdentifier != "io.cloudeng.sample" {
		t.Errorf("CFBundleIdentifier = %q, want io.cloudeng.sample", readInfo.CFBundleIdentifier)
	}
	if readInfo.LSMinimumSystemVersion != "15.0" {
		t.Errorf("LSMinimumSystemVersion = %q, want 15.0", readInfo.LSMinimumSystemVersion)
	}

	// Non-existent plist
	if _, err := buildtools.ReadInfoPlist(filepath.Join(tempDir, "nonexistent.plist")); err == nil {
		t.Error("expected non-existent plist to fail")
	}
	_ = plistPath
}
