// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Git struct {
	dir string
}

// NewGit creates a new Git instance rooted at the specified directory which
// must be within a git repository.
func NewGit(dir string) Git {
	return Git{dir: dir}
}

func (g Git) Hash(ctx context.Context, cmdRunner *CommandRunner, branch string, n int) (StepResult, error) {
	if len(branch) == 0 {
		branch = "HEAD"
	}
	ctx = ContextWithCWD(ctx, g.dir)
	if n == 0 {
		n = 8
	}
	short := fmt.Sprintf("--short=%d", n)
	return cmdRunner.Run(ctx, "git", "rev-parse", short, branch)
}

// Version is the version of a bundle built from a git repository, see
// Git.Version.
type Version struct {
	// Short is the human-readable release version, for
	// CFBundleShortVersionString.
	Short string
	// Build is Short with the abbreviated commit appended, as
	// <short>+<commit>, and -dirty if the tree has uncommitted changes, for
	// CFBundleVersion.
	Build string
	// Commit is the full hash of the commit that HEAD refers to.
	Commit string
	// BuildTime is when the version was determined, in UTC.
	BuildTime time.Time
	// Dirty reports whether the tree has uncommitted changes, in which case
	// the bundle does not match Commit.
	Dirty bool
}

// Version returns the version of a bundle built from the repository's
// current tree, whose release version is short: its HEAD commit, and whether
// it has uncommitted changes. It is for the caller to decide whether a dirty
// tree is acceptable.
func (g Git) Version(ctx context.Context, cmdRunner *CommandRunner, short string) (Version, error) {
	v := Version{Short: short, Build: short, BuildTime: time.Now().UTC()}
	full, err := g.Hash(ctx, cmdRunner, "HEAD", 40)
	if err != nil {
		return v, fmt.Errorf("determining the git revision of %v: %w", g.dir, err)
	}
	v.Commit = strings.TrimSpace(full.Output())
	abbrev, err := g.Hash(ctx, cmdRunner, "HEAD", 8)
	if err != nil {
		return v, fmt.Errorf("determining the short git revision of %v: %w", g.dir, err)
	}
	v.Build = short + "+" + strings.TrimSpace(abbrev.Output())
	status, err := cmdRunner.Run(ContextWithCWD(ctx, g.dir), "git", "status", "--porcelain")
	if err != nil {
		return v, fmt.Errorf("determining whether %v is clean: %w", g.dir, err)
	}
	if v.Dirty = strings.TrimSpace(status.Output()) != ""; v.Dirty {
		v.Build += "-dirty"
	}
	return v, nil
}

func (g Git) key() string {
	return "+git:"
}

func (g Git) GetBranch(version string) string {
	index := strings.Index(version, g.key())
	if index == -1 {
		return ""
	}
	return version[index+len(g.key()):]
}

func (g Git) ReplaceBranch(version, buildID string) string {
	index := strings.Index(version, g.key())
	if index == -1 {
		return version
	}
	return version[:index] + "+" + buildID
}
