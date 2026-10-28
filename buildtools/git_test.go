// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cloudeng.io/macos/buildtools"
)

func TestGitGetBranch(t *testing.T) {
	git := buildtools.NewGit("/tmp")

	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "version with git key and branch",
			version:  "1.0.0+git:main",
			expected: "main",
		},
		{
			name:     "version with git key and HEAD",
			version:  "2.0.0+git:HEAD",
			expected: "HEAD",
		},
		{
			name:     "version without git key",
			version:  "1.0.0",
			expected: "",
		},
		{
			name:     "version with git key and feature branch",
			version:  "1.5.0+git:feature/new-feature",
			expected: "feature/new-feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := git.GetBranch(tt.version)
			if result != tt.expected {
				t.Errorf("GetBranch(%q) = %q, want %q", tt.version, result, tt.expected)
			}
		})
	}
}

func TestGitReplaceBranch(t *testing.T) {
	git := buildtools.NewGit("/tmp")

	tests := []struct {
		name     string
		version  string
		buildID  string
		expected string
	}{
		{
			name:     "version with git key",
			version:  "1.0.0+git:main",
			buildID:  "abc12345",
			expected: "1.0.0+abc12345",
		},
		{
			name:     "version without git key",
			version:  "1.0.0",
			buildID:  "abc12345",
			expected: "1.0.0",
		},
		{
			name:     "version with git key and HEAD",
			version:  "2.0.0+git:HEAD",
			buildID:  "def67890",
			expected: "2.0.0+def67890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := git.ReplaceBranch(tt.version, tt.buildID)
			if result != tt.expected {
				t.Errorf("ReplaceBranch(%q, %q) = %q, want %q", tt.version, tt.buildID, result, tt.expected)
			}
		})
	}
}

func initGitRepo(ctx context.Context, t *testing.T, runner *buildtools.CommandRunner, _ string) {
	_, err := runner.Run(ctx, "git", "init")
	if err != nil {
		t.Skipf("git not available, skipping test: %v", err)
	}

	// Configure git user for the test
	_, err = runner.Run(ctx, "git", "config", "user.name", "Test User")
	if err != nil {
		t.Skipf("failed to configure git user.name: %v", err)
	}
	_, err = runner.Run(ctx, "git", "config", "user.email", "test@example.com")
	if err != nil {
		t.Skipf("failed to configure git user.email: %v", err)
	}
}

func verifyHexString(t *testing.T, hash string) {
	for _, c := range hash {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("hash contains invalid character %q: %q", string(c), hash)
		}
	}
}

func TestGitHash(t *testing.T) {
	// Create a temporary directory for our test git repository
	tempDir := t.TempDir()

	// Initialize a git repository for testing
	ctx := t.Context()
	runner := buildtools.NewCommandRunner()
	ctx = buildtools.ContextWithCWD(ctx, tempDir)

	initGitRepo(ctx, t, runner, tempDir)

	fatal := func(err error, format string, args ...any) {
		if err != nil {
			t.Fatalf(format, args...)
		}
	}

	// Create a test file and commit it
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0600)
	fatal(err, "failed to create test file: %v", err)

	_, err = runner.Run(ctx, "git", "add", "test.txt")
	fatal(err, "failed to add file to git: %v", err)

	_, err = runner.Run(ctx, "git", "commit", "-m", "Initial commit")
	fatal(err, "failed to commit: %v", err)

	// Create Git instance
	git := buildtools.NewGit(tempDir)

	// Test Hash function
	result, err := git.Hash(ctx, runner, "HEAD", 8)
	fatal(err, "failed to get git hash: %v", err)

	if result.Error() != nil {
		t.Fatalf("git hash command failed: %s", result.Error())
	}

	hash := strings.TrimSpace(result.Output())
	if len(hash) != 8 {
		t.Errorf("expected hash length of 8, got %d: %q", len(hash), hash)
	}

	// Verify it's a valid hex string
	verifyHexString(t, hash)

	// Test with different hash length
	result12, err := git.Hash(ctx, runner, "HEAD", 12)
	fatal(err, "failed to get git hash with length 12: %v", err)

	if result12.Error() != nil {
		t.Fatalf("git hash command failed: %s", result12.Error())
	}

	hash12 := strings.TrimSpace(result12.Output())
	if len(hash12) != 12 {
		t.Errorf("expected hash length of 12, got %d: %q", len(hash12), hash12)
	}

	// Verify shorter hash is prefix of longer hash
	if !strings.HasPrefix(hash12, hash) {
		t.Errorf("expected shorter hash %q to be prefix of longer hash %q", hash, hash12)
	}

	// Test with default length (0 should use 8)
	resultDefault, err := git.Hash(ctx, runner, "HEAD", 0)
	fatal(err, "failed to get git hash with default length: %v", err)

	if resultDefault.Error() != nil {
		t.Fatalf("git hash command failed: %s", resultDefault.Error())
	}

	hashDefault := strings.TrimSpace(resultDefault.Output())
	if len(hashDefault) != 8 {
		t.Errorf("expected default hash length of 8, got %d: %q", len(hashDefault), hashDefault)
	}

	if hashDefault != hash {
		t.Errorf("expected default hash %q to equal explicit 8-char hash %q", hashDefault, hash)
	}

	// Test with empty branch (should default to HEAD)
	resultEmpty, err := git.Hash(ctx, runner, "", 8)
	fatal(err, "failed to get git hash with empty branch: %v", err)

	if resultEmpty.Error() != nil {
		t.Fatalf("git hash command failed: %s", resultEmpty.Error())
	}

	hashEmpty := strings.TrimSpace(resultEmpty.Output())
	if hashEmpty != hash {
		t.Errorf("expected empty branch hash %q to equal HEAD hash %q", hashEmpty, hash)
	}
}

// commitAll initializes a repository in dir, with a file in sub, a
// subdirectory, and commits it.
func commitAll(t *testing.T, runner *buildtools.CommandRunner, dir, sub string) {
	t.Helper()
	ctx := buildtools.ContextWithCWD(t.Context(), dir)
	initGitRepo(ctx, t, runner, dir)
	if err := os.MkdirAll(sub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "initial"}} {
		if _, err := runner.Run(ctx, "git", args...); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGitVersion(t *testing.T) {
	tempDir := t.TempDir()
	runner := buildtools.NewCommandRunner()
	sub := filepath.Join(tempDir, "sub")
	commitAll(t, runner, tempDir, sub)

	// A subdirectory of the repository will do.
	git := buildtools.NewGit(sub)
	v, err := git.Version(t.Context(), runner, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if v.Short != "1.2.3" || len(v.Commit) != 40 || v.Dirty || v.BuildTime.IsZero() || v.BuildTime.Location() != time.UTC {
		t.Errorf("clean: got %+v", v)
	}
	if want := "1.2.3+" + v.Commit[:8]; v.Build != want {
		t.Errorf("clean: build %q, want %q", v.Build, want)
	}
	verifyHexString(t, v.Commit)

	if err := os.WriteFile(filepath.Join(tempDir, "b.txt"), []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	dirty, err := git.Version(t.Context(), runner, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if !dirty.Dirty || dirty.Commit != v.Commit || dirty.Build != v.Build+"-dirty" {
		t.Errorf("dirty: got %+v", dirty)
	}

	if _, err := buildtools.NewGit(t.TempDir()).Version(t.Context(), runner, "1.2.3"); err == nil {
		t.Error("expected an error for a directory that is not in a repository")
	}
}
