// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"cloudeng.io/macos/buildtools"
	"gopkg.in/yaml.v3"
)

const plistYAML = `
CFBundleIdentifier: io.cloudeng.TestApp
CFBundleName: TestApp
CFBundleVersion: 1.0.0
CFBundleShortVersionString: 1.0
CFBundleExecutable: TestExecutable
CFBundlePackageType: APPL
LSMinimumSystemVersion: "15.0"
CFBundleDisplayName: Swift UI Example
`

func TestAppBundle(t *testing.T) {
	// Create a temporary directory for our test
	tempDir := t.TempDir()
	var info buildtools.InfoPlist
	if err := yaml.Unmarshal([]byte(plistYAML), &info); err != nil {
		t.Fatalf("failed to unmarshal info plist: %v", err)
	}

	// Define a simple app bundle
	bundle := buildtools.AppBundle{
		Path: filepath.Join(tempDir, "TestApp.app"),
		Info: info,
	}

	// Create a command runner for executing steps
	runner := buildtools.NewCommandRunner()
	ctx := context.Background()

	// Execute the steps to create the bundle
	steps := bundle.Create()
	if len(steps) == 0 {
		t.Fatal("expected steps to create bundle, but got none")
	}
	steps = append(steps, bundle.WriteInfoPlist())

	// Execute each step
	for i, step := range steps {
		_, err := step.Run(ctx, runner)
		if err != nil {
			t.Fatalf("step %d failed with error: %v", i, err)
		}
	}

	// Verify the bundle structure
	requiredPaths := []string{
		bundle.Path,
		filepath.Join(bundle.Path, "Contents"),
		filepath.Join(bundle.Path, "Contents", "MacOS"),
		filepath.Join(bundle.Path, "Contents", "Resources"),
		filepath.Join(bundle.Path, "Contents", "Info.plist"),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected path %q to exist, but it doesn't: %v", path, err)
		}
	}

	// Test copying content
	// Create a test file to copy into the bundle
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Copy the file into the bundle's Resources directory
	copyStep := bundle.CopyContents(testFile, "Resources", "test.txt")
	if _, err := copyStep.Run(ctx, runner); err != nil {
		t.Fatalf("copy step failed: %v", err)
	}

	// Verify the file was copied
	copiedPath := filepath.Join(bundle.Path, "Contents", "Resources", "test.txt")
	if _, err := os.Stat(copiedPath); err != nil {
		t.Fatalf("expected file %q to exist, but it doesn't: %v", copiedPath, err)
	}
}

func TestWriteInfoPlistGitBuild(t *testing.T) {
	tempDir := t.TempDir()
	var info buildtools.InfoPlist
	if err := yaml.Unmarshal([]byte(plistYAML), &info); err != nil {
		t.Fatalf("failed to unmarshal info plist: %v", err)
	}

	bundle := buildtools.AppBundle{
		Path: filepath.Join(tempDir, "TestApp.app"),
		Info: info,
	}

	runner := buildtools.NewCommandRunner()
	ctx := context.Background()

	for _, step := range bundle.Create() {
		if _, err := step.Run(ctx, runner); err != nil {
			t.Fatalf("bundle create failed: %v", err)
		}
	}

	// Case 1: CFBundleVersion has no branch pattern ("1.0.0").
	// Previously this deadlocked because versionCh was never closed.
	git := buildtools.NewGit(".")
	steps := bundle.WriteInfoPlistGitBuild(ctx, git)
	for i, step := range steps {
		if _, err := step.Run(ctx, runner); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}

	// Case 2: CFBundleVersion specifies branch ("1.0.0+git:HEAD")
	bundleWithGit := bundle
	bundleWithGit.Info.CFBundleVersion = "1.0.0+git:HEAD"
	gitSteps := bundleWithGit.WriteInfoPlistGitBuild(ctx, git)
	for i, step := range gitSteps {
		if _, err := step.Run(ctx, runner); err != nil {
			t.Fatalf("git step %d failed: %v", i, err)
		}
	}
}

func TestAppBundlePermissions(t *testing.T) {
	tempDir := t.TempDir()
	var info buildtools.InfoPlist
	if err := yaml.Unmarshal([]byte(plistYAML), &info); err != nil {
		t.Fatalf("failed to unmarshal info plist: %v", err)
	}

	bundle := buildtools.AppBundle{
		Path: filepath.Join(tempDir, "TestApp.app"),
		Info: info,
	}

	runner := buildtools.NewCommandRunner()
	ctx := context.Background()

	for _, step := range bundle.Create() {
		if _, err := step.Run(ctx, runner); err != nil {
			t.Fatalf("bundle create failed: %v", err)
		}
	}

	// Create a dummy executable file
	exePath := filepath.Join(bundle.Path, "Contents", "MacOS", info.CFBundleExecutable)
	if err := os.WriteFile(exePath, []byte("#!/bin/sh\necho ok\n"), 0600); err != nil {
		t.Fatalf("failed to create dummy executable: %v", err)
	}

	// Set executable permissions to 0700
	stepExe := bundle.SetExecutablePermissions("", 0700)
	if _, err := stepExe.Run(ctx, runner); err != nil {
		t.Fatalf("SetExecutablePermissions failed: %v", err)
	}
	fi, err := os.Stat(exePath)
	if err != nil {
		t.Fatalf("stat executable failed: %v", err)
	}
	if got, want := fi.Mode().Perm(), fs.FileMode(0700); got != want {
		t.Errorf("executable permissions = %04o, want %04o", got, want)
	}

	// Set MacOS dir permissions to 0750
	dirPath := filepath.Join(bundle.Path, "Contents", "MacOS")
	stepDir := bundle.SetMacOSDirPermissions(0750)
	if _, err := stepDir.Run(ctx, runner); err != nil {
		t.Fatalf("SetMacOSDirPermissions failed: %v", err)
	}
	dirFi, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("stat MacOS dir failed: %v", err)
	}
	if got, want := dirFi.Mode().Perm(), fs.FileMode(0750); got != want {
		t.Errorf("MacOS dir permissions = %04o, want %04o", got, want)
	}

	// Passing dirFi.Mode() directly (which includes os.ModeDir) should succeed
	stepDirMode := bundle.SetMacOSDirPermissions(dirFi.Mode())
	if _, err := stepDirMode.Run(ctx, runner); err != nil {
		t.Fatalf("SetMacOSDirPermissions with ModeDir failed: %v", err)
	}
}

func TestAppBundleSetExecutablePermissionsErrors(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := context.Background()

	// Error case: both CFBundleExecutable and src are empty
	emptyBundle := buildtools.AppBundle{
		Path: filepath.Join(tempDir, "Empty.app"),
	}
	stepEmpty := emptyBundle.SetExecutablePermissions("", 0700)
	if _, err := stepEmpty.Run(ctx, runner); err == nil {
		t.Fatal("expected SetExecutablePermissions to fail when both CFBundleExecutable and src are empty")
	}

	// Fallback case: CFBundleExecutable is empty, but src is provided
	bundle := buildtools.AppBundle{
		Path: filepath.Join(tempDir, "Fallback.app"),
	}
	for _, step := range bundle.Create() {
		if _, err := step.Run(ctx, runner); err != nil {
			t.Fatalf("bundle create failed: %v", err)
		}
	}
	customExe := filepath.Join(bundle.Path, "Contents", "MacOS", "custom_bin")
	if err := os.WriteFile(customExe, []byte("#!/bin/sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stepFallback := bundle.SetExecutablePermissions("/path/to/custom_bin", 0755)
	if _, err := stepFallback.Run(ctx, runner); err != nil {
		t.Fatalf("SetExecutablePermissions with src fallback failed: %v", err)
	}
	customFi, err := os.Stat(customExe)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := customFi.Mode().Perm(), fs.FileMode(0755); got != want {
		t.Errorf("custom_bin permissions = %04o, want %04o", got, want)
	}
}

func setupTestAppBundle(t *testing.T) (buildtools.AppBundle, string) {
	t.Helper()
	tempDir := t.TempDir()
	bundle := buildtools.AppBundle{
		Path: filepath.Join(tempDir, "MyApp.app"),
		Info: buildtools.InfoPlist{
			CFBundleIdentifier: "com.example.myapp",
			CFBundleExecutable: "myapp",
			CFBundleIconFile:   "AppIcon.icns",
		}.WithDefaults("myapp"),
	}
	ctx := t.Context()
	runner := buildtools.NewCommandRunner()
	for _, s := range bundle.Create() {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatal(err)
		}
	}
	return bundle, tempDir
}

func TestAppBundlePathsAndFiles(t *testing.T) {
	bundle, _ := setupTestAppBundle(t)

	if got, want := bundle.ExecutablePath(), filepath.Join(bundle.Path, "Contents", "MacOS", "myapp"); got != want {
		t.Errorf("ExecutablePath = %q, want %q", got, want)
	}
	if got, want := bundle.Contents("Frameworks", "lib.dylib"), filepath.Join(bundle.Path, "Contents", "Frameworks", "lib.dylib"); got != want {
		t.Errorf("Contents = %q, want %q", got, want)
	}
	if got, want := bundle.Resources("config.json"), filepath.Join(bundle.Path, "Contents", "Resources", "config.json"); got != want {
		t.Errorf("Resources = %q, want %q", got, want)
	}
}

func TestAppBundleCopyExecutableAndProfile(t *testing.T) {
	bundle, tempDir := setupTestAppBundle(t)
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	srcExe := filepath.Join(tempDir, "built_myapp")
	if err := os.WriteFile(srcExe, []byte("binary"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.CopyExecutable(srcExe).Run(ctx, runner); err != nil {
		t.Fatalf("CopyExecutable failed: %v", err)
	}
	if _, err := os.Stat(bundle.ExecutablePath()); err != nil {
		t.Errorf("expected copied executable to exist: %v", err)
	}
	if _, err := bundle.CopyExecutable("").Run(ctx, runner); err == nil {
		t.Error("expected CopyExecutable with empty src to fail")
	}

	profFile := filepath.Join(tempDir, "embedded.mobileprovision")
	if err := os.WriteFile(profFile, []byte("profile"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.InstallProvisioningProfile(profFile).Run(ctx, runner); err != nil {
		t.Fatalf("InstallProvisioningProfile failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle.Path, "Contents", "embedded.provisionprofile")); err != nil {
		t.Errorf("expected embedded profile to exist: %v", err)
	}
	if _, err := bundle.InstallProvisioningProfile("").Run(ctx, runner); err == nil {
		t.Error("expected InstallProvisioningProfile with empty path to fail")
	}
}

func TestAppBundleCopyIcons(t *testing.T) {
	bundle, tempDir := setupTestAppBundle(t)
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	dummyIconFile := filepath.Join(tempDir, "AppIcon.icns")
	if err := os.WriteFile(dummyIconFile, []byte("icns"), 0600); err != nil {
		t.Fatal(err)
	}
	iconSet := buildtools.IconSet{Dir: tempDir, Name: "AppIcon.icns"}
	if _, err := bundle.CopyIcons(iconSet)[0].Run(ctx, runner); err != nil {
		t.Fatalf("CopyIcons failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle.Path, "Contents", "Resources", "AppIcon.icns")); err != nil {
		t.Errorf("expected AppIcon.icns in Resources: %v", err)
	}
	if _, err := bundle.CopyIcons()[0].Run(ctx, runner); err != nil {
		t.Errorf("expected empty CopyIcons to be noop: %v", err)
	}
	noIconBundle := bundle
	noIconBundle.Info.CFBundleIconFile = ""
	if _, err := noIconBundle.CopyIcons(iconSet)[0].Run(ctx, runner); err != nil {
		t.Errorf("expected CopyIcons with no CFBundleIconFile to be noop: %v", err)
	}
}

func TestAppBundleSigningAndClean(t *testing.T) {
	bundle, _ := setupTestAppBundle(t)
	runner := buildtools.NewCommandRunner()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	ctx := t.Context()

	signer := buildtools.NewSigner("Developer ID Application: Test", nil, nil, nil)
	signSteps := []buildtools.Step{
		bundle.Sign(signer),
		bundle.SignExecutable(signer),
		bundle.SignContents(signer, "Resources", "AppIcon.icns"),
		bundle.VerifyContents(signer, "Resources", "AppIcon.icns"),
		bundle.SPCtlAsses(),
	}
	for i, s := range signSteps {
		res, err := s.Run(ctx, dryRunner)
		if err != nil {
			t.Fatalf("signStep %d failed: %v", i, err)
		}
		if res.Executable() == "" {
			t.Errorf("step %d executable should not be empty", i)
		}
	}
	for _, s := range bundle.VerifySignatures(signer) {
		if _, err := s.Run(ctx, dryRunner); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := bundle.SignContents(signer).Run(ctx, runner); err == nil {
		t.Error("expected SignContents with empty dst to fail")
	}
	if _, err := bundle.VerifyContents(signer).Run(ctx, runner); err == nil {
		t.Error("expected VerifyContents with empty dst to fail")
	}

	for _, s := range bundle.Clean() {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatalf("clean step failed: %v", err)
		}
	}
	if _, err := os.Stat(bundle.Path); !os.IsNotExist(err) {
		t.Errorf("expected bundle %q to be removed by Clean", bundle.Path)
	}
}

func TestAppBundleSymlinkExecutable(t *testing.T) {
	bundle, tempDir := setupTestAppBundle(t)
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	srcExe := filepath.Join(tempDir, "built_myapp")
	if err := os.WriteFile(srcExe, []byte("binary"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.CopyExecutable(srcExe).Run(ctx, runner); err != nil {
		t.Fatalf("CopyExecutable failed: %v", err)
	}

	linkPath := filepath.Join(tempDir, "myapp-link")
	if _, err := bundle.SymlinkExecutable(linkPath).Run(ctx, runner); err != nil {
		t.Fatalf("SymlinkExecutable failed: %v", err)
	}
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat failed: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink, got %v", fi.Mode())
	}
	dest, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	// Verify that reading through the symlink resolves to the copied executable.
	content, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("reading through symlink failed: %v", err)
	}
	if string(content) != "binary" {
		t.Errorf("got content %q, want %q", string(content), "binary")
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(linkPath), dest)
	}
	if dest != bundle.ExecutablePath() {
		t.Errorf("dest = %q, want %q", dest, bundle.ExecutablePath())
	}

	// Test overwriting existing link
	if _, err := bundle.SymlinkExecutable(linkPath).Run(ctx, runner); err != nil {
		t.Fatalf("SymlinkExecutable overwrite failed: %v", err)
	}

	// Empty link should error
	if _, err := bundle.SymlinkExecutable("").Run(ctx, runner); err == nil {
		t.Error("expected empty link to fail")
	}

	// Empty CFBundleExecutable should error
	emptyBundle := bundle
	emptyBundle.Info.CFBundleExecutable = ""
	if _, err := emptyBundle.SymlinkExecutable(linkPath).Run(ctx, runner); err == nil {
		t.Error("expected empty CFBundleExecutable to fail")
	}
}
