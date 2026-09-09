// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"os"
	"path/filepath"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func setupTestPkgBuild(t *testing.T) (buildtools.PkgBuild, string) {
	t.Helper()
	tempDir := t.TempDir()
	pb := buildtools.PkgBuild{
		BuildDir:        filepath.Join(tempDir, "pkg_build"),
		Identifier:      "com.example.testpkg",
		Version:         "1.0.0",
		InstallLocation: "/Applications/TestApp.app",
	}
	return pb, tempDir
}

func TestPkgBuildCreateAndPaths(t *testing.T) {
	pb, _ := setupTestPkgBuild(t)
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	createSteps := pb.Create()
	if len(createSteps) != 4 {
		t.Fatalf("expected 4 create steps, got %d", len(createSteps))
	}
	for i, s := range createSteps {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatalf("create step %d failed: %v", i, err)
		}
	}
	for _, sub := range []string{"", "root/Applications", "outputs", "scripts"} {
		path := filepath.Join(pb.BuildDir, sub)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected directory %q to exist: %v", path, err)
		}
	}
	if got, want := pb.ScriptsPath(), filepath.Join(pb.BuildDir, "scripts"); got != want {
		t.Errorf("ScriptsPath = %q, want %q", got, want)
	}
	if got, want := pb.OutputsPath(), filepath.Join(pb.BuildDir, "outputs"); got != want {
		t.Errorf("OutputsPath = %q, want %q", got, want)
	}
	if got, want := pb.LibraryPath("myLib"), filepath.Join(pb.BuildDir, "root", "Library", "myLib"); got != want {
		t.Errorf("LibraryPath = %q, want %q", got, want)
	}
	if _, err := pb.CreateLibrary("myLib").Run(ctx, runner); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}
	if _, err := os.Stat(pb.LibraryPath("myLib")); err != nil {
		t.Errorf("expected library dir to exist: %v", err)
	}
}

func TestPkgBuildScriptsAndPlist(t *testing.T) {
	pb, _ := setupTestPkgBuild(t)
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	for _, s := range pb.Create() {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatal(err)
		}
	}

	scriptData := []byte("#!/bin/sh\necho test\n")
	if _, err := pb.WriteScript(scriptData, "postinstall").Run(ctx, runner); err != nil {
		t.Fatalf("WriteScript failed: %v", err)
	}
	writtenScript, err := os.ReadFile(filepath.Join(pb.ScriptsPath(), "postinstall"))
	if err != nil || string(writtenScript) != string(scriptData) {
		t.Errorf("WriteScript content mismatch: %v", err)
	}
	if _, err := pb.WriteScript(nil, "name").Run(ctx, runner); err != nil {
		t.Errorf("expected empty data to be noop: %v", err)
	}
	if _, err := pb.WriteScript(scriptData, "").Run(ctx, runner); err == nil {
		t.Error("expected empty script name to fail")
	}

	compPlist := []buildtools.PkgComponentPlist{
		{
			RootRelativeBundlePath: "Applications/TestApp.app",
			BundleIsRelocatable:    false,
		},
	}
	if _, err := pb.WritePlist(compPlist).Run(ctx, runner); err != nil {
		t.Fatalf("WritePlist failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pb.BuildDir, "component.plist")); err != nil {
		t.Errorf("expected component.plist to exist: %v", err)
	}
}

func TestPkgBuildCopyResources(t *testing.T) {
	pb, tempDir := setupTestPkgBuild(t)
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	for _, s := range pb.Create() {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatal(err)
		}
	}

	appSrc := filepath.Join(tempDir, "dummy.app")
	if err := os.MkdirAll(appSrc, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := pb.CopyApplication(appSrc).Run(ctx, runner); err != nil {
		t.Fatalf("CopyApplication failed: %v", err)
	}
	if _, err := pb.CopyApplication("").Run(ctx, runner); err != nil {
		t.Errorf("CopyApplication empty should be noop: %v", err)
	}

	scriptsSrc := filepath.Join(tempDir, "dummy_scripts")
	if err := os.MkdirAll(scriptsSrc, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := pb.CopyScripts(scriptsSrc).Run(ctx, runner); err != nil {
		t.Fatalf("CopyScripts failed: %v", err)
	}
	if _, err := pb.CopyScripts("").Run(ctx, runner); err != nil {
		t.Errorf("CopyScripts empty should be noop: %v", err)
	}

	libSrc := filepath.Join(tempDir, "dummy_lib")
	if err := os.MkdirAll(libSrc, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := pb.CopyLibrary(libSrc, "myLib").Run(ctx, runner); err != nil {
		t.Fatalf("CopyLibrary failed: %v", err)
	}
	if _, err := pb.CopyLibrary("", "myLib").Run(ctx, runner); err != nil {
		t.Errorf("CopyLibrary empty should be noop: %v", err)
	}
}

func TestPkgBuildBuildAndInstall(t *testing.T) {
	pb, _ := setupTestPkgBuild(t)
	runner := buildtools.NewCommandRunner()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	ctx := t.Context()

	buildStep := pb.Build("output.pkg")
	res, err := buildStep.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("Build dry-run failed: %v", err)
	}
	if res.Executable() != "pkgbuild" {
		t.Errorf("expected pkgbuild executable, got %q", res.Executable())
	}

	incompletePB := pb
	incompletePB.Identifier = ""
	if _, err := incompletePB.Build("output.pkg").Run(ctx, runner); err == nil {
		t.Error("expected Build with empty Identifier to fail")
	}

	installStep := pb.Install("output.pkg")
	res, err = installStep.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("Install dry-run failed: %v", err)
	}
	if res.Executable() != "sudo" {
		t.Errorf("expected sudo executable, got %q", res.Executable())
	}
	incompleteInstallPB := pb
	incompleteInstallPB.InstallLocation = ""
	if _, err := incompleteInstallPB.Install("output.pkg").Run(ctx, runner); err == nil {
		t.Error("expected Install with empty InstallLocation to fail")
	}
}

func TestPkgBuildClean(t *testing.T) {
	pb, _ := setupTestPkgBuild(t)
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	for _, s := range pb.Create() {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatal(err)
		}
	}

	cleanStep := pb.Clean()
	if _, err := cleanStep.Run(ctx, runner); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}
	if _, err := os.Stat(pb.BuildDir); !os.IsNotExist(err) {
		t.Errorf("expected %q to be removed", pb.BuildDir)
	}

	for _, badDir := range []string{"", "/", "."} {
		badPB := buildtools.PkgBuild{BuildDir: badDir}
		if _, err := badPB.Clean().Run(ctx, runner); err == nil {
			t.Errorf("expected Clean with bad dir %q to fail", badDir)
		}
	}
}
