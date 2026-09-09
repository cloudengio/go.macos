// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools_test

import (
	"path/filepath"
	"testing"

	"cloudeng.io/macos/buildtools"
)

func TestIconSizeMultipleSuffix(t *testing.T) {
	for _, tc := range []struct {
		m    buildtools.IconSizeMultiple
		want string
	}{
		{buildtools.IconSize1x, ""},
		{buildtools.IconSize2x, "2x"},
		{buildtools.IconSize3x, "3x"},
	} {
		if got := tc.m.Suffix(); got != tc.want {
			t.Errorf("IconSizeMultiple(%d).Suffix() = %q, want %q", int(tc.m), got, tc.want)
		}
	}
}

func TestIconSetDefaultsAndCustom(t *testing.T) {
	// 1. Defaults
	isDefault := buildtools.IconSet{}
	if got, want := isDefault.IconSetDir(), "Icons.iconset"; got != want {
		t.Errorf("IconSetDir = %q, want %q", got, want)
	}
	if got, want := isDefault.IconSetName(), "AppIcon.icns"; got != want {
		t.Errorf("IconSetName = %q, want %q", got, want)
	}
	if got, want := isDefault.IconSetFile(), filepath.Join("Icons.iconset", "AppIcon.icns"); got != want {
		t.Errorf("IconSetFile = %q, want %q", got, want)
	}
	if got, want := isDefault.IconFormat(), "png"; got != want {
		t.Errorf("IconFormat = %q, want %q", got, want)
	}

	// 2. Custom
	isCustom := buildtools.IconSet{
		Dir:    "Custom.iconset",
		Name:   "Custom.icns",
		Format: "tiff",
		Sizes:  []int{16, 32},
	}
	if got, want := isCustom.IconSetDir(), "Custom.iconset"; got != want {
		t.Errorf("IconSetDir = %q, want %q", got, want)
	}
	if got, want := isCustom.IconSetName(), "Custom.icns"; got != want {
		t.Errorf("IconSetName = %q, want %q", got, want)
	}
	if got, want := isCustom.IconFormat(), "tiff"; got != want {
		t.Errorf("IconFormat = %q, want %q", got, want)
	}
}

func TestIconSetVariantsAndSteps(t *testing.T) {
	ctx := t.Context()
	runner := buildtools.NewCommandRunner(buildtools.WithDryRun(true))

	isCustom := buildtools.IconSet{
		Dir:    "Custom.iconset",
		Name:   "Custom.icns",
		Format: "tiff",
		Sizes:  []int{16, 32},
	}

	// CreateIconVariants (dry-run)
	steps := isCustom.CreateIconVariants("source.png", "Custom.iconset")
	// Sizes: 16 (1x, 2x, 3x) + 32 (1x, 2x, 3x) + 1 (icns) = 7 steps
	if len(steps) != 7 {
		t.Fatalf("expected 7 steps, got %d", len(steps))
	}
	for i, s := range steps {
		res, err := s.Run(ctx, runner)
		if err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
		if res.Executable() == "" {
			t.Errorf("step %d executable should not be empty", i)
		}
	}

	// CreateIcns
	icnsStep := isCustom.CreateIcns()
	res, err := icnsStep.Run(ctx, runner)
	if err != nil {
		t.Fatalf("CreateIcns failed: %v", err)
	}
	if res.Executable() != "iconutil" {
		t.Errorf("executable = %q, want iconutil", res.Executable())
	}

	// ReformatIcon
	reformat := buildtools.ReformatIcon{
		InputPath:  "in.jpg",
		OutputPath: "out.png",
	}
	res, err = reformat.Convert("png").Run(ctx, runner)
	if err != nil {
		t.Fatalf("ReformatIcon.Convert failed: %v", err)
	}
	if res.Executable() != "sips" {
		t.Errorf("executable = %q, want sips", res.Executable())
	}

	// IconSetSteps from resources.go
	resObj := buildtools.Resources{
		Icons: []buildtools.IconSet{isCustom},
	}
	resSteps := resObj.IconSetSteps()
	if len(resSteps) == 0 {
		t.Fatal("expected non-empty icon set steps")
	}
	for i, s := range resSteps {
		if _, err := s.Run(ctx, runner); err != nil {
			t.Fatalf("resStep %d failed: %v", i, err)
		}
	}
}
