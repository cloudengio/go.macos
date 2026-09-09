// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// AppBundle represents a macOS application bundle.
// See: https://developer.apple.com/documentation/bundleresources
// See: https://developer.apple.com/documentation/bundleresources/placing-content-in-a-bundle
type AppBundle struct {
	Path string
	Info InfoPlist
}

// Create returns the steps required to create the app bundle directory structure
// and Info.plist.
func (b AppBundle) Create() []Step {
	steps := []Step{
		MkdirAll(b.Path),
		MkdirAll(filepath.Join(b.Path, "Contents", "MacOS")),
		MkdirAll(filepath.Join(b.Path, "Contents", "Resources")),
	}
	return steps
}

func (b AppBundle) WriteInfoPlistGitBuild(_ context.Context, git Git) []Step {
	versionCh := make(chan string, 1)

	getHash := StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		defer close(versionCh)
		branch := git.GetBranch(b.Info.CFBundleVersion)
		if len(branch) == 0 {
			return StepResult{}, nil
		}
		res, err := git.Hash(ctx, cmdRunner, branch, 8)
		if err != nil {
			return res, err
		}
		newVersion := git.ReplaceBranch(b.Info.CFBundleVersion, strings.TrimSpace(res.Output()))
		if b.Info.CFBundleVersion == newVersion {
			return NewStepResult("no change to CFBundleVersion", nil, nil, nil), nil
		}
		versionCh <- newVersion
		return NewStepResult(
			fmt.Sprintf("CFBundleVersion: replace %q with %q", branch, newVersion), nil, nil, nil), nil
	})

	writePlist := StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		newVersion, ok := <-versionCh
		if !ok {
			return NewStepResult("no new version for CFBundleVersion, skipping update", nil, nil, nil), nil
		}
		// b is a value receiver, so this updates only this step's copy.
		b.Info.CFBundleVersion = newVersion
		return writeInfoPlist(filepath.Join(b.Path, "Contents", "Info.plist"), b.Info).Run(ctx, cmdRunner)
	})

	return []Step{getHash, writePlist}
}

// WriteInfoPlist returns the step required to write the Info.plist file for the app bundle.
func (b AppBundle) WriteInfoPlist() Step {
	if err := b.Info.Validate(); err != nil {
		return ErrorStep(err, "writeInfoPlist", filepath.Join(b.Path, "Contents", "Info.plist"))
	}
	return writeInfoPlist(filepath.Join(b.Path, "Contents", "Info.plist"), b.Info)
}

// CopyContents returns the step required to copy a file into the app bundle
// dst is relative to the bundle Contents root.
func (b AppBundle) CopyContents(src string, dst ...string) Step {
	p := filepath.Join(dst...)
	if src == "" || len(p) == 0 {
		return ErrorStep(fmt.Errorf("source (%q) or destination (%q) not specified", src, dst), "cp", src, p)
	}
	return Copy(src, filepath.Join(b.Path, "Contents", p))
}

// CopyExecutable returns the step required to copy the executable referenced
// in the Info.plist into the app bundle.
func (b AppBundle) CopyExecutable(src string) Step {
	if src == "" {
		return ErrorStep(fmt.Errorf("source executable path not specified"), "cp", src, "")
	}
	dst := filepath.Join(b.Path, "Contents", "MacOS", b.Info.CFBundleExecutable)
	return Copy(src, dst)
}

// SetExecutablePermissions returns a Step that sets the permissions of the
// executable referenced in the Info.plist within the app bundle. If the
// permissions are 0, it returns a NoopStep. Otherwise, if Info.CFBundleExecutable
// is not set and src is provided, filepath.Base(src) is used. If both are empty,
// it returns an ErrorStep.
func (b AppBundle) SetExecutablePermissions(src string, perms fs.FileMode) Step {
	if perms == 0 {
		return NoopStep("no permissions specified, skipping chmod")
	}
	exe := b.Info.CFBundleExecutable
	if exe == "" && src != "" {
		exe = filepath.Base(src)
	}
	if exe == "" {
		return ErrorStep(fmt.Errorf("executable not specified in Info.plist or src"), "chmod", fmt.Sprintf("%o", chmodBits(perms)))
	}
	dst := filepath.Join(b.Path, "Contents", "MacOS", exe)
	return Perms(dst, perms)
}

// SetMacOSDirPermissions returns a Step that sets the permissions of the MacOS
// directory. If the permissions are 0, it returns a NoopStep.
func (b AppBundle) SetMacOSDirPermissions(perms fs.FileMode) Step {
	if perms == 0 {
		return NoopStep("no permissions specified, skipping chmod")
	}
	dst := filepath.Join(b.Path, "Contents", "MacOS")
	return Perms(dst, perms)
}

// SignExecutable returns the step required to sign the executable within the app bundle.
func (b AppBundle) SignExecutable(signer Signer) Step {
	return signer.SignPath(b.Path, filepath.Join("Contents", "MacOS", b.Info.CFBundleExecutable))
}

// ExecutablePath returns the absolute path to the executable inside
// the app bundle that is referenced in the Info.plist.
func (b AppBundle) ExecutablePath() string {
	return filepath.Join(b.Path, "Contents", "MacOS", b.Info.CFBundleExecutable)
}

// InstallProvisioningProfile returns a Step that copies the provisioning profile
// into the app bundle.
// See https://developer.apple.com/documentation/technotes/tn3125-inside-code-signing-provisioning-profiles for an explanation of
// provisioning profiles.
func (b AppBundle) InstallProvisioningProfile(profile string) Step {
	if profile == "" {
		return ErrorStep(fmt.Errorf("provisioning profile path not specified"), "cp", profile, "")
	}
	dst := filepath.Join(b.Path, "Contents", "embedded.provisionprofile")
	return Copy(profile, dst)
}

// Clean returns Steps that removes the app bundle directory and all its contents.
// The permissions of the app bundle directory are set to 0700 before removal to
// ensure that it can be deleted.
func (b AppBundle) Clean() []Step {
	return []Step{
		ChmodAll(b.Path, 0700),
		RmdirAll(b.Path)}
}

// SignContents returns the step required to sign a file within the app bundle,
// dst is relative to the bundle Contents root.
func (b AppBundle) SignContents(signer Signer, dst ...string) Step {
	p := filepath.Join(dst...)
	if len(p) == 0 {
		return ErrorStep(fmt.Errorf("destination (%q) not specified", dst), "codesign", "", p)
	}
	return signer.SignPath(b.Path, filepath.Join("Contents", p))
}

// VerifyContents returns the step required to sign a file within the app bundle,
// dst is relative to the bundle Contents root.
func (b AppBundle) VerifyContents(signer Signer, dst ...string) Step {
	p := filepath.Join(dst...)
	if len(p) == 0 {
		return ErrorStep(fmt.Errorf("destination (%q) not specified", dst), "codesign", "", p)
	}
	return signer.VerifyPath(b.Path, filepath.Join("Contents", p))
}

// Contents returns the path to the specified element within the app bundle's
// Contents directory.
func (b AppBundle) Contents(elem ...string) string {
	return filepath.Join(b.Path, "Contents", filepath.Join(elem...))
}

// Resources returns the path to the specified element within the app bundle's
// Resources directory.
func (b AppBundle) Resources(elem ...string) string {
	return filepath.Join(b.Path, "Contents", "Resources", filepath.Join(elem...))
}

// CopyIcons returns steps to copy the specified icons into the app bundle's
// Resources directory. If multiple icons are specified and the icon's BundleIcon
// field is set or if there is only a single icon then it is copied to the location
// specified by the bundle's Info.plist CFBundleIconFile field. All other icons
// are copied to their own directories within the Resources directory.
func (b AppBundle) CopyIcons(icons ...IconSet) []Step {
	if len(icons) == 0 {
		return []Step{NoopStep("CopyIcons: no icons specified for the bundle")}
	}
	if len(b.Info.CFBundleIconFile) == 0 {
		return []Step{NoopStep("CopyIcons: bundle Info.plist CFBundleIconFile not set")}
	}
	steps := []Step{}
	for _, icon := range icons {
		var dst string
		if icon.BundleIcon || len(icons) == 1 {
			dst = filepath.Join(b.Path, "Contents", "Resources", b.Info.CFBundleIconFile)
		} else {
			dst = filepath.Join(b.Path, "Contents", "Resources", icon.Name)
		}
		steps = append(steps, Copy(icon.IconSetFile(), dst))
	}
	return steps
}

func (b AppBundle) Sign(signer Signer) Step {
	return signer.SignPath(b.Path, "")
}

func (b AppBundle) VerifySignatures(signer Signer) []Step {
	steps := []Step{
		signer.VerifyPath(b.Path, ""),
	}
	return steps
}

func (b AppBundle) SPCtlAsses() Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "spctl", "--assess", "--type", "execute", b.Path)
	})
}
