// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestFileOperations(t *testing.T) {
	// Create a temporary directory for our test
	tempDir := t.TempDir()

	// Create a command runner for executing steps
	runner := buildtools.NewCommandRunner()
	ctx := context.Background()

	// Test directory creation
	testDirPath := filepath.Join(tempDir, "test_dir", "nested")
	mkdirStep := buildtools.MkdirAll(testDirPath)

	result, err := mkdirStep.Run(ctx, runner)
	if err != nil {
		t.Fatalf("mkdir step failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(testDirPath); err != nil {
		t.Fatalf("expected directory %q to exist, but it doesn't: %v", testDirPath, err)
	}

	// Test file creation
	testContent := []byte("test content")
	testFilePath := filepath.Join(tempDir, "test_file.txt")
	if err := os.WriteFile(testFilePath, testContent, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test file copy
	destPath := filepath.Join(testDirPath, "copied_file.txt")
	copyStep := buildtools.Copy(testFilePath, destPath)

	result, err = copyStep.Run(ctx, runner)
	if err != nil {
		t.Fatalf("copy step failed: %v", err)
	}

	// Verify file was copied
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected file %q to exist, but it doesn't: %v", destPath, err)
	}

	// Verify content is correct
	copiedContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(copiedContent) != string(testContent) {
		t.Fatalf("copied content does not match original, got %q, want %q", string(copiedContent), string(testContent))
	}

	// Test command execution with StepFunc
	cmdStep := buildtools.StepFunc(func(_ context.Context, _ *buildtools.CommandRunner) (buildtools.StepResult, error) {
		return buildtools.NewStepResult("test command", []string{"arg1", "arg2"}, []byte("test output"), nil), nil
	})

	result, err = cmdStep.Run(ctx, runner)
	if err != nil {
		t.Fatalf("command step failed: %v", err)
	}

	if result.Executable() != "test command" {
		t.Fatalf("command mismatch, got %q, want %q", result.Executable(), "test command")
	}
	if len(result.Args()) != 2 || result.Args()[0] != "arg1" || result.Args()[1] != "arg2" {
		t.Fatalf("Args mismatch, got %v, want %v", result.Args(), []string{"arg1", "arg2"})
	}
	if result.Output() != "test output" {
		t.Fatalf("output mismatch, got %q, want %q", result.Output(), "test output")
	}
}

func TestPermsWithFileModeTypeBits(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := context.Background()

	// 1. Directory with os.ModeDir set
	testDir := filepath.Join(tempDir, "test_dir")
	if err := os.Mkdir(testDir, 0700); err != nil {
		t.Fatal(err)
	}
	dirFi, err := os.Stat(testDir)
	if err != nil {
		t.Fatal(err)
	}
	if dirFi.Mode()&os.ModeDir == 0 {
		t.Fatal("expected ModeDir bit to be set")
	}

	// Passing FileMode with os.ModeDir set should not fail chmod
	step := buildtools.Perms(testDir, dirFi.Mode())
	if _, err := step.Run(ctx, runner); err != nil {
		t.Fatalf("Perms with ModeDir failed: %v", err)
	}

	// 2. File with os.ModeDir bit explicitly added (simulating callers passing ModeDir)
	testFile := filepath.Join(tempDir, "test_file.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	stepFile := buildtools.Perms(testFile, os.ModeDir|0700)
	if _, err := stepFile.Run(ctx, runner); err != nil {
		t.Fatalf("Perms on file with ModeDir bit failed: %v", err)
	}
	fileFi, err := os.Stat(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fileFi.Mode().Perm(), os.FileMode(0700); got != want {
		t.Errorf("file perms = %04o, want %04o", got, want)
	}
}

func TestRmdirAll(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	// 1. Valid .app removal
	appDir := filepath.Join(tempDir, "Test.app")
	if err := os.MkdirAll(filepath.Join(appDir, "Contents", "MacOS"), 0700); err != nil {
		t.Fatal(err)
	}
	step := buildtools.RmdirAll(appDir)
	if _, err := step.Run(ctx, runner); err != nil {
		t.Fatalf("RmdirAll failed: %v", err)
	}
	if _, err := os.Stat(appDir); !os.IsNotExist(err) {
		t.Errorf("expected %q to be removed", appDir)
	}

	// 2. Valid .app with trailing slash
	appDirSlash := filepath.Join(tempDir, "Slash.app")
	if err := os.MkdirAll(appDirSlash, 0700); err != nil {
		t.Fatal(err)
	}
	stepSlash := buildtools.RmdirAll(appDirSlash + "/")
	if _, err := stepSlash.Run(ctx, runner); err != nil {
		t.Fatalf("RmdirAll with trailing slash failed: %v", err)
	}
	if _, err := os.Stat(appDirSlash); !os.IsNotExist(err) {
		t.Errorf("expected %q to be removed", appDirSlash)
	}

	// 3. Non-existent .app directory (should be noop / succeed)
	stepNonExistent := buildtools.RmdirAll(filepath.Join(tempDir, "NonExistent.app"))
	if _, err := stepNonExistent.Run(ctx, runner); err != nil {
		t.Fatalf("RmdirAll on non-existent app should succeed: %v", err)
	}

	// 4. Non-.app directory (should return ErrorStep)
	nonAppDir := filepath.Join(tempDir, "regular_dir")
	stepNonApp := buildtools.RmdirAll(nonAppDir)
	if _, err := stepNonApp.Run(ctx, runner); err == nil {
		t.Error("expected RmdirAll on non-.app directory to fail")
	}
}

func TestChmodAll(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	// 1. Valid .app chmod
	appDir := filepath.Join(tempDir, "ChmodTest.app")
	if err := os.MkdirAll(filepath.Join(appDir, "Contents"), 0700); err != nil {
		t.Fatal(err)
	}
	step := buildtools.ChmodAll(appDir, 0755)
	if _, err := step.Run(ctx, runner); err != nil {
		t.Fatalf("ChmodAll failed: %v", err)
	}

	// 2. Trailing slash
	stepSlash := buildtools.ChmodAll(appDir+"/", 0700)
	if _, err := stepSlash.Run(ctx, runner); err != nil {
		t.Fatalf("ChmodAll with trailing slash failed: %v", err)
	}

	// 3. Non-existent .app (noop)
	stepNonExistent := buildtools.ChmodAll(filepath.Join(tempDir, "NonExistent.app"), 0755)
	if _, err := stepNonExistent.Run(ctx, runner); err != nil {
		t.Fatalf("ChmodAll on non-existent app should succeed: %v", err)
	}

	// 4. Non-.app directory (error)
	stepNonApp := buildtools.ChmodAll(filepath.Join(tempDir, "dir"), 0755)
	if _, err := stepNonApp.Run(ctx, runner); err == nil {
		t.Error("expected ChmodAll on non-.app directory to fail")
	}
}

func TestFileAndDirExists(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	// Directory exists
	if _, err := buildtools.DirExists(tempDir).Run(ctx, runner); err != nil {
		t.Errorf("DirExists failed for existing directory: %v", err)
	}
	if _, err := buildtools.DirExists(filepath.Join(tempDir, "nonexistent")).Run(ctx, runner); err == nil {
		t.Error("DirExists should fail for non-existent directory")
	}

	// File exists
	testFile := filepath.Join(tempDir, "exists.txt")
	if err := os.WriteFile(testFile, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildtools.FileExists(testFile).Run(ctx, runner); err != nil {
		t.Errorf("FileExists failed for existing file: %v", err)
	}
	if _, err := buildtools.FileExists(filepath.Join(tempDir, "missing.txt")).Run(ctx, runner); err == nil {
		t.Error("FileExists should fail for non-existent file")
	}
}

func TestFileOperationsRenameAndCopy(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	// MkdirAll with empty name
	if _, err := buildtools.MkdirAll("").Run(ctx, runner); err == nil {
		t.Error("expected MkdirAll with empty path to fail")
	}

	// Rename
	srcFile := filepath.Join(tempDir, "old.txt")
	dstFile := filepath.Join(tempDir, "new.txt")
	if err := os.WriteFile(srcFile, []byte("rename test"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildtools.Rename(srcFile, dstFile).Run(ctx, runner); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	if _, err := os.Stat(dstFile); err != nil {
		t.Errorf("expected renamed file to exist: %v", err)
	}

	// CopyDir
	srcDir := filepath.Join(tempDir, "src_dir")
	dstDir := filepath.Join(tempDir, "dst_dir")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "file.txt"), []byte("nested"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildtools.CopyDir(srcDir, dstDir).Run(ctx, runner); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "sub", "file.txt")); err != nil {
		t.Errorf("expected copied dir file to exist: %v", err)
	}

	// RSync
	rsyncDst := filepath.Join(tempDir, "rsync_dst")
	if err := os.MkdirAll(rsyncDst, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := buildtools.RSync(srcDir+"/", rsyncDst).Run(ctx, runner); err != nil {
		t.Fatalf("RSync failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rsyncDst, "sub", "file.txt")); err != nil {
		t.Errorf("expected rsync synced file to exist: %v", err)
	}
}

func TestFileOperationsWriting(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	// WriteFile dry-run and live
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	writeFile := filepath.Join(tempDir, "write.txt")
	if _, err := buildtools.WriteFile([]byte("hello"), 0644, writeFile).Run(ctx, dryRunner); err != nil {
		t.Fatalf("WriteFile dry run failed: %v", err)
	}
	if _, err := os.Stat(writeFile); !os.IsNotExist(err) {
		t.Errorf("dry-run should not write file")
	}
	if _, err := buildtools.WriteFile([]byte("hello"), 0644, writeFile).Run(ctx, runner); err != nil {
		t.Fatalf("WriteFile live failed: %v", err)
	}
	data, err := os.ReadFile(writeFile)
	if err != nil || string(data) != "hello" {
		t.Errorf("WriteFile content mismatch: %v, %s", err, string(data))
	}

	// WriteJSONFile
	jsonFile := filepath.Join(tempDir, "test.json")
	val := map[string]string{"foo": "bar"}
	if _, err := buildtools.WriteJSONFile(val, jsonFile).Run(ctx, runner); err != nil {
		t.Fatalf("WriteJSONFile failed: %v", err)
	}
	if _, err := os.Stat(jsonFile); err != nil {
		t.Errorf("expected json file to exist: %v", err)
	}

	// WritePlistFile
	plistFile := filepath.Join(tempDir, "test.plist")
	plistVal := buildtools.InfoPlist{CFBundleIdentifier: "com.test"}.WithDefaults("test")
	if _, err := buildtools.WritePlistFile(plistVal, plistFile).Run(ctx, runner); err != nil {
		t.Fatalf("WritePlistFile failed: %v", err)
	}
	if _, err := os.Stat(plistFile); err != nil {
		t.Errorf("expected plist file to exist: %v", err)
	}
}

func TestIsValidIconSetDir(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewCommandRunner()

	validStep := buildtools.IsValidIconSetDir("MyIcons.iconset")
	if _, err := validStep.Run(ctx, runner); err != nil {
		t.Errorf("expected .iconset to be valid: %v", err)
	}

	invalidStep := buildtools.IsValidIconSetDir("MyIcons.png")
	if _, err := invalidStep.Run(ctx, runner); err == nil {
		t.Error("expected .png to fail IsValidIconSetDir")
	}
}

func TestSymlink(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	target := filepath.Join(tempDir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(tempDir, "link.txt")
	if _, err := buildtools.Symlink(target, link).Run(ctx, runner); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat failed: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink mode, got %v", fi.Mode())
	}

	// Overwrite existing symlink
	if _, err := buildtools.Symlink(target, link).Run(ctx, runner); err != nil {
		t.Fatalf("Symlink overwrite failed: %v", err)
	}

	// Empty arguments should error
	if _, err := buildtools.Symlink("", link).Run(ctx, runner); err == nil {
		t.Error("expected empty target to fail")
	}
	if _, err := buildtools.Symlink(target, "").Run(ctx, runner); err == nil {
		t.Error("expected empty link to fail")
	}
}
