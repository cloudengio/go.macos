// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func isDirWritable(dir string) bool {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	return unix.Access(dir, unix.W_OK) == nil
}

func goEnvGOBIN(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "go", "env", "GOBIN")
	out, err := cmd.Output()
	if err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	return strings.TrimSpace(os.Getenv("GOBIN"))
}

// InstallDir returns the first directory that exists and is writable from the
// candidate directories. If searchDirs is empty, it checks the value of
// 'go env GOBIN' followed by each directory in the PATH environment variable.
func InstallDir(ctx context.Context, searchDirs ...string) (string, error) {
	var candidates []string
	if len(searchDirs) > 0 {
		candidates = searchDirs
	} else {
		if gobin := goEnvGOBIN(ctx); gobin != "" {
			candidates = append(candidates, gobin)
		}
		for _, p := range filepath.SplitList(os.Getenv("PATH")) {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				candidates = append(candidates, trimmed)
			}
		}
	}

	seen := make(map[string]bool)
	for _, dir := range candidates {
		clean := filepath.Clean(dir)
		if clean == "" || clean == "." || seen[clean] {
			continue
		}
		seen[clean] = true
		if abs, err := filepath.Abs(clean); err == nil {
			clean = abs
		}
		if isDirWritable(clean) {
			return clean, nil
		}
	}
	return "", fmt.Errorf("no existing and writable directory found in candidate paths: %v", candidates)
}

// Install returns a Step that installs the app bundle into the first directory
// that exists and is writable, starting with the value of 'go env GOBIN'
// followed by the members of PATH (or searchDirs if specified).
// If softlink is true, a symbolic link to the bundle's main executable
// is created in the install directory with the same name as the executable.
func (b AppBundle) Install(softlink bool, searchDirs ...string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		if b.Path == "" {
			return ErrorStep(fmt.Errorf("bundle path not specified"), "install").Run(ctx, cmdRunner)
		}
		if _, err := os.Stat(b.Path); err != nil && !cmdRunner.DryRun() {
			return ErrorStep(fmt.Errorf("bundle not found: %w", err), "install", b.Path).Run(ctx, cmdRunner)
		}
		installDir, err := InstallDir(ctx, searchDirs...)
		if err != nil {
			return ErrorStep(err, "install", b.Path).Run(ctx, cmdRunner)
		}

		cleanSrc := filepath.Clean(b.Path)
		dstBundle := filepath.Join(installDir, filepath.Base(cleanSrc))

		if cleanSrc != dstBundle {
			src := strings.TrimSuffix(cleanSrc, "/") + "/"
			rsyncStep := RSync(src, dstBundle)
			if res, err := rsyncStep.Run(ctx, cmdRunner); err != nil {
				return res, fmt.Errorf("failed to copy bundle to %s: %w", dstBundle, err)
			}
		}

		if softlink {
			exe := b.Info.CFBundleExecutable
			if exe == "" {
				plistPath := filepath.Join(cleanSrc, "Contents", "Info.plist")
				if info, err := ReadInfoPlist(plistPath); err == nil && info.CFBundleExecutable != "" {
					exe = info.CFBundleExecutable
				}
			}
			if exe == "" {
				return ErrorStep(fmt.Errorf("executable not specified in Info.plist"), "ln", "-s", "-f").Run(ctx, cmdRunner)
			}
			link := filepath.Join(installDir, exe)
			installed := AppBundle{Path: dstBundle, Info: InfoPlist{CFBundleExecutable: exe}}
			symlinkStep := installed.SymlinkExecutable(link)
			if res, err := symlinkStep.Run(ctx, cmdRunner); err != nil {
				return res, fmt.Errorf("failed to create symlink at %s: %w", link, err)
			}
		}

		return NewStepResult("install", []string{b.Path, dstBundle}, nil, nil), nil
	})
}
