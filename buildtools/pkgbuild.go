// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"fmt"
	"path/filepath"
)

// PkgBuild represents the pkgbuild tool and its configuration.
type PkgBuild struct {
	BuildDir        string `yaml:"build_dir"`        // Directory to use for building the package
	Identifier      string `yaml:"identifier"`       // Package identifier, e.g. com.cloudeng.myapp
	Version         string `yaml:"version"`          // Package version, e.g. 1.0.0
	InstallLocation string `yaml:"install_location"` // Installation location, e.g. /Applications/MyApp.app

}

// Clean returns a Step that removes the BuildDir directory.
func (p PkgBuild) Clean() Step {
	cleanDir := filepath.Clean(p.BuildDir)
	if len(p.BuildDir) == 0 || cleanDir == "/" || cleanDir == "." {
		return ErrorStep(fmt.Errorf("invalid build dir specified: %q", p.BuildDir), "rm", "-rf", p.BuildDir)
	}
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "rm", "-rf", p.BuildDir)
	})
}

// Create returns the steps required to create the pkgbuild directory structure.
func (p PkgBuild) Create() []Step {
	steps := []Step{
		MkdirAll(p.BuildDir),
		MkdirAll(filepath.Join(p.BuildDir, "root", "Applications")),
		MkdirAll(filepath.Join(p.BuildDir, "outputs")),
		MkdirAll(filepath.Join(p.BuildDir, "scripts")),
	}
	return steps

}

// CopyApplication returns a Step that copies the specified application bundle to the
// Applications directory within the package build root.
func (p PkgBuild) CopyApplication(src string) Step {
	if len(src) == 0 {
		return NoopStep("no application specified")
	}
	return RSync(src, filepath.Join(p.BuildDir, "root", "Applications"))
}

// CopyScripts returns a Step that copies the specified scripts directory to the
// scripts directory within the package build root.
func (p PkgBuild) CopyScripts(src string) Step {
	if len(src) == 0 {
		return NoopStep("no scripts specified")
	}
	return RSync(src+"/", filepath.Join(p.BuildDir, "scripts"))
}

// WriteScript returns a Step that writes the specified script data to a file
// with the given name in the scripts directory within the package build root.
func (p PkgBuild) WriteScript(data []byte, name string) Step {
	if len(data) == 0 {
		return NoopStep("no script data specified")
	}
	if len(name) == 0 {
		return ErrorStep(fmt.Errorf("no script name specified"), "write script")
	}
	return WriteFile(data, 0500, filepath.Join(p.BuildDir, "scripts", name))
}

// CopyLibrary returns a Step that copies the specified library directory to the
// Library directory within the package build root.
// Note that this is one way of installing files for use by the Installer.
func (p PkgBuild) CopyLibrary(src, library string) Step {
	if len(src) == 0 {
		return NoopStep("no library specified")
	}
	return RSync(src+"/", filepath.Join(p.BuildDir, "root", "Library", library))
}

// WritePlist returns a Step that writes the specified component plist configuration
// to the component.plist file within the package build root.
func (p PkgBuild) WritePlist(cfg []PkgComponentPlist) Step {
	return writeInfoPlist(filepath.Join(p.BuildDir, "component.plist"), cfg)
}

// ScriptsPath returns the path to the scripts directory within the package build root.
func (p PkgBuild) ScriptsPath() string {
	return filepath.Join(p.BuildDir, "scripts")
}

// CreateLibrary returns a Step that creates the specified library directory
func (p PkgBuild) CreateLibrary(library string) Step {
	return MkdirAll(filepath.Join(p.BuildDir, "root", "Library", library))
}

// LibraryPath returns the path to the specified library directory within the package build root.
func (p PkgBuild) LibraryPath(library string) string {
	return filepath.Join(p.BuildDir, "root", "Library", library)
}

// OutputsPath returns the path to the outputs directory within the package build root.
func (p PkgBuild) OutputsPath() string {
	return filepath.Join(p.BuildDir, "outputs")
}

// Build returns a Step that builds the package using pkgbuild.
func (p PkgBuild) Build(outputPath string) Step {
	if len(outputPath) == 0 || len(p.InstallLocation) == 0 || len(p.Identifier) == 0 || len(p.Version) == 0 {
		return ErrorStep(fmt.Errorf("one of outputPath, InstallLocation, Identifier or Version is not set: %+v", p), "pkgbuild")
	}
	pkgPath := filepath.Join(p.OutputsPath(), filepath.Base(outputPath))
	args := []string{
		"--root", filepath.Join(p.BuildDir, "root"),
		"--component-plist", filepath.Join(p.BuildDir, "component.plist"),
		"--identifier", p.Identifier,
		"--version", p.Version,
		"--install-location", p.InstallLocation,
		"--scripts", p.ScriptsPath(),
		pkgPath,
	}
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "pkgbuild", args...)
	})
}

// Install returns a Step that installs the package using the system installer command
// using sudo.
func (p PkgBuild) Install(outputPath string) Step {
	if len(p.InstallLocation) == 0 {
		return ErrorStep(fmt.Errorf("no install location specified"), "open")
	}
	pkgPath := filepath.Join(p.OutputsPath(), filepath.Base(outputPath))
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "sudo", "installer", "-pkg", pkgPath, "-target", p.InstallLocation)
	})
}

// PkgConfPkgComponentPlist represents the pkgbuild component plist structure.
type PkgComponentPlist struct {
	RootRelativeBundlePath      string `yaml:"RootRelativeBundlePath" plist:"RootRelativeBundlePath"`
	BundleIsRelocatable         bool   `yaml:"BundleIsRelocatable" plist:"BundleIsRelocatable,omitempty"`
	BundleIsVersionChecked      bool   `yaml:"BundleIsVersionChecked" plist:"BundleIsVersionChecked,omitempty"`
	BundleHasStrictIdentifier   bool   `yaml:"BundleHasStrictIdentifier" plist:"BundleHasStrictIdentifier,omitempty"`
	BundleOverwriteAction       string `yaml:"BundleOverwriteAction" plist:"BundleOverwriteAction,omitempty"`
	BundlePreInstallScriptPath  string `yaml:"BundlePreInstallScriptPath" plist:"BundlePreInstallScriptPath,omitempty"`
	BundlePostInstallScriptPath string `yaml:"BundlePostInstallScriptPath" plist:"BundlePostInstallScriptPath,omitempty"`
	BundleInstallScriptTimeout  int    `yaml:"BundleInstallScriptTimeout" plist:"BundleInstallScriptTimeout,omitempty"`
}
