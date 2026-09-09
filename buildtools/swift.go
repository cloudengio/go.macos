// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"path/filepath"
	"strings"
)

// SwiftApp represents the swift build tool.
type SwiftApp struct {
	root    string
	release bool
	bindir  string
}

// binDir returns the directory containing the swift build products.
func (s SwiftApp) binDir(ctx context.Context) (string, error) {
	runner := NewCommandRunner()
	args := []string{"build", "--show-bin-path"}
	if s.release {
		args = append(args, "--configuration", "release")
	}
	ctx = ContextWithCWD(ctx, s.root)
	r, err := runner.Run(ctx, "swift", args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(r.Output()), nil
}

// NewSwiftApp creates a new SwiftApp instance rooted at the specified directory.
func NewSwiftApp(ctx context.Context, root string, release bool) SwiftApp {
	sw := SwiftApp{root: root, release: release}
	var err error
	sw.bindir, err = sw.binDir(ctx)
	if err != nil {
		panic(err)
	}
	return sw
}

// Build returns a Step that builds the swift project.
func (s SwiftApp) Build() Step {
	args := []string{"build"}
	if s.release {
		args = append(args, "--configuration", "release")
	}
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		ctx = ContextWithCWD(ctx, s.root)
		return cmdRunner.Run(ctx, "swift", args...)
	})
}

// BinDir returns the directory containing the swift build products.
func (s SwiftApp) BinDir() string {
	return s.bindir
}

// CopyIcons returns steps to copy the specified icons into the swift build
// tree's Resources directory.
func (s SwiftApp) CopyIcons(icons []IconSet) []Step {
	steps := []Step{}
	for _, icon := range icons {
		dst := filepath.Join(s.root, "Resources", icon.IconSetName())
		steps = append(steps, Copy(icon.IconSetFile(), dst))
	}
	return steps
}

// ExecutablePath returns the path to the specified executable within the
// swift build tree.
func (s SwiftApp) ExecutablePath(name string) string {
	return filepath.Join(s.BinDir(), name)
}
