// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"os"
	"path/filepath"
	"testing"

	"cloudeng.io/macos/buildtools"
	"gopkg.in/yaml.v3"
)

func TestProductBuildCreateAndResources(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	ctx := t.Context()

	prod := buildtools.ProductBuild{
		PkgBuild: buildtools.PkgBuild{
			BuildDir:   filepath.Join(tempDir, "prod_build"),
			Identifier: "com.example.prod",
			Version:    "1.0.0",
		},
		InstallLocation: "/",
		GUIXML:          "distribution.xml",
	}

	createSteps := prod.Create()
	for i, s := range createSteps {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatalf("create step %d failed: %v", i, err)
		}
	}
	if _, err := os.Stat(filepath.Join(prod.BuildDir, "resources")); err != nil {
		t.Errorf("expected resources dir to exist: %v", err)
	}

	emptyProd := buildtools.ProductBuild{}
	if _, err := emptyProd.Create()[0].Run(ctx, runner); err == nil {
		t.Error("expected Create with empty BuildDir to fail")
	}

	if got, want := prod.ResourcesPath(), filepath.Join(prod.BuildDir, "resources"); got != want {
		t.Errorf("ResourcesPath = %q, want %q", got, want)
	}

	resFile := filepath.Join(tempDir, "res.txt")
	if err := os.WriteFile(resFile, []byte("resource"), 0600); err != nil {
		t.Fatal(err)
	}
	copySteps := prod.CopyResources(resFile)
	for _, s := range copySteps {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatalf("CopyResources failed: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(prod.ResourcesPath(), "res.txt")); err != nil {
		t.Errorf("expected copied resource to exist: %v", err)
	}
	if _, err := prod.CopyResources()[0].Run(ctx, runner); err != nil {
		t.Errorf("expected empty CopyResources to be noop: %v", err)
	}
}

func TestProductBuildDistributionAndInstall(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))
	ctx := t.Context()

	prod := buildtools.ProductBuild{
		PkgBuild: buildtools.PkgBuild{
			BuildDir:   filepath.Join(tempDir, "prod_build"),
			Identifier: "com.example.prod",
			Version:    "1.0.0",
		},
		InstallLocation: "/",
		GUIXML:          "distribution.xml",
	}

	step := prod.BuildDistribution("output.pkg", "Developer ID Installer: Example")
	res, err := step.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("BuildDistribution dry-run failed: %v", err)
	}
	if res.Executable() != "productbuild" {
		t.Errorf("expected productbuild executable, got %q", res.Executable())
	}

	stepNoSign := prod.BuildDistribution("output.pkg", "")
	res, err = stepNoSign.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("BuildDistribution without sign dry-run failed: %v", err)
	}
	if res.Executable() != "productbuild" {
		t.Errorf("expected productbuild executable, got %q", res.Executable())
	}

	if _, err := prod.BuildDistribution("", "").Run(ctx, runner); err == nil {
		t.Error("expected BuildDistribution with empty output path to fail")
	}
	noXMLProd := prod
	noXMLProd.GUIXML = ""
	if _, err := noXMLProd.BuildDistribution("output.pkg", "").Run(ctx, runner); err == nil {
		t.Error("expected BuildDistribution with empty GUIXML to fail")
	}
	noDirProd := prod
	noDirProd.BuildDir = ""
	if _, err := noDirProd.BuildDistribution("output.pkg", "").Run(ctx, runner); err == nil {
		t.Error("expected BuildDistribution with empty BuildDir to fail")
	}

	installStep := prod.Install("output.pkg")
	res, err = installStep.Run(ctx, dryRunner)
	if err != nil {
		t.Fatalf("Install dry-run failed: %v", err)
	}
	if res.Executable() != "sudo" {
		t.Errorf("expected sudo executable, got %q", res.Executable())
	}
	noLocProd := prod
	noLocProd.InstallLocation = ""
	if _, err := noLocProd.Install("output.pkg").Run(ctx, runner); err == nil {
		t.Error("expected Install with empty InstallLocation to fail")
	}
}

func TestProductPreInstallRequirements(t *testing.T) {
	yamlData := `
foo: bar
num: 42
`
	var req buildtools.ProductPreInstallRequirements
	if err := yaml.Unmarshal([]byte(yamlData), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.Raw["foo"] != "bar" {
		t.Errorf("foo = %v, want bar", req.Raw["foo"])
	}

	marshaledYAML, err := req.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML failed: %v", err)
	}
	if marshaledYAML == nil {
		t.Error("expected non-nil YAML")
	}

	marshaledPlist, err := req.MarshalPlist()
	if err != nil {
		t.Fatalf("MarshalPlist failed: %v", err)
	}
	if marshaledPlist == nil {
		t.Error("expected non-nil Plist")
	}
}
