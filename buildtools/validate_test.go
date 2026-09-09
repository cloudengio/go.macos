// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestSuffixAssert(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewCommandRunner()
	dryRunner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))

	s := buildtools.Suffix(".app")

	// 1. Dry run
	if _, err := s.Assert("foo.app").Run(ctx, dryRunner); err != nil {
		t.Errorf("expected dry-run to succeed: %v", err)
	}

	// 2. Matching suffix
	if _, err := s.Assert("foo.app").Run(ctx, runner); err != nil {
		t.Errorf("expected matching suffix to succeed: %v", err)
	}

	// 3. Mismatching suffix
	if _, err := s.Assert("foo.txt").Run(ctx, runner); err == nil {
		t.Error("expected mismatching suffix to fail")
	}
}
