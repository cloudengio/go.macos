// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProductBuild represents the productbuild tool.
type ProductBuild struct {
	PkgBuild
	InstallLocation string // target location for the install, e.g. /
	GUIXML          string // path to the distribution XML file relative to the resources directory
}

// Create returns steps that create the product build directory structure
// in addition to those created by the embedded PkgBuild.
func (p ProductBuild) Create() []Step {
	if len(p.BuildDir) == 0 {
		return []Step{ErrorStep(fmt.Errorf("no build dir specified"), "mkdir", "-p")}
	}
	return []Step{
		MkdirAll(filepath.Join(p.BuildDir, "resources")),
	}
}

// CopyResources returns a Step that copies the specified resource to the
// resources directory within the product build root.
func (p ProductBuild) CopyResources(src ...string) []Step {
	if len(src) == 0 {
		return []Step{NoopStep("no resource specified")}
	}
	steps := make([]Step, 0, len(src))
	for _, s := range src {
		steps = append(steps, Copy(s, filepath.Join(p.BuildDir, "resources")))
	}
	return steps
}

// ResourcesPath returns the path to the resources directory.
func (p ProductBuild) ResourcesPath() string {
	return filepath.Join(p.BuildDir, "resources")
}

// BuildDistribution returns a Step that creates a product archive using productbuild
// with the specified distribution XML at outputPkgPath.
func (p ProductBuild) BuildDistribution(outputPkgPath, signingIdentity string) Step {
	if len(p.BuildDir) == 0 {
		return ErrorStep(fmt.Errorf("no build dir specified"), "productbuild")
	}
	if len(p.GUIXML) == 0 {
		return ErrorStep(fmt.Errorf("no distribution XML specified"), "productbuild")
	}
	if len(outputPkgPath) == 0 {
		return ErrorStep(fmt.Errorf("no output path specified"), "productbuild")
	}
	args := []string{
		"--distribution", filepath.Join(p.BuildDir, "resources", p.GUIXML),
		"--package-path", p.OutputsPath(),
		"--resources", p.ResourcesPath(),
	}
	if len(signingIdentity) != 0 {
		args = append(args, "--sign", signingIdentity)
	}
	args = append(args, outputPkgPath)
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "productbuild", args...)
	})
}

// Install returns a Step that installs the package using the system installer command.
func (p ProductBuild) Install(outputPath string) Step {
	if len(p.InstallLocation) == 0 {
		return ErrorStep(fmt.Errorf("no install location specified"), "installer")
	}
	pkgPath := filepath.Join(p.OutputsPath(), filepath.Base(outputPath))
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "sudo", "installer", "-pkg", pkgPath, "-target", p.InstallLocation)
	})
}

// ProductBuildResources represents the resources needed to create a productbuild
// distribution.
type ProductBuildResources struct {
	GUIXML          string   `yaml:"gui_xml"`          // The distribution XML file.
	SigningIdentity string   `yaml:"signing_identity"` // The signing identity to use for signing the product.
	Resources       []string `yaml:"resources"`        // paths to additional resources to include in the product build.
	Packages        []string `yaml:"packages"`         // paths to the component packages to include in the product build.
}

// ProductPreInstallRequirements represents the productbuild pre-install requirements
// for synthesized packages.
type ProductPreInstallRequirements struct {
	Raw map[string]any
}

func (p ProductPreInstallRequirements) MarshalYAML() (any, error) {
	return p.Raw, nil
}

func (p *ProductPreInstallRequirements) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&p.Raw)
}

func (p ProductPreInstallRequirements) MarshalPlist() (any, error) {
	return p.Raw, nil
}
